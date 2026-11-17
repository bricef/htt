#!/usr/bin/env bash
# Mint an htt release: the steps in docs/releasing.md, in order,
# stopping at the first that fails. Run it through just:
#
#   just release-check v0.4.0        # every check; changes nothing
#   just release v0.4.0 notes.md     # check, tag, publish, verify
#
# RELEASE_DRY_RUN=1 prints the commands that would tag and publish
# instead of running them (and skips verifying the install).
set -euo pipefail

repo=bricef/htt
module=github.com/bricef/htt
govulncheck=golang.org/x/vuln/cmd/govulncheck@v1.9.0 # the version CI runs

usage() {
	echo "usage: release.sh --check-only vX.Y.Z | release.sh vX.Y.Z NOTES.md" >&2
	exit 2
}
step() { printf '\n==> %s\n' "$*"; }
fail() {
	printf 'release: %s\n' "$*" >&2
	exit 1
}
dry_run=${RELEASE_DRY_RUN:-}
run() { # a command that changes something outside this checkout
	if [[ -n $dry_run ]]; then
		printf '[dry run] %s\n' "$*"
	else
		"$@"
	fi
}

check_only=false
if [[ ${1:-} == --check-only ]]; then
	check_only=true
	shift
fi
version=${1:-}
notes=${2:-}
[[ -n $version ]] || usage
$check_only || [[ -n $notes ]] || usage

step "Version $version"
[[ $version =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] ||
	fail "$version isn't a version tag like v1.2.3"
git fetch --quiet --tags origin
if git rev-parse --quiet --verify "refs/tags/$version" >/dev/null; then
	fail "$version is already tagged; tags are permanent, so pick the next version"
fi
previous=$(git tag --list 'v*' --sort=-v:refname | head -1)
if [[ -n $previous ]]; then
	newest=$(printf '%s\n%s\n' "$previous" "$version" | sort -V | tail -1)
	[[ $newest == "$version" ]] || fail "$version isn't newer than the latest release, $previous"
fi
echo "the release after ${previous:-nothing (the first release)}"

step "The commit is main as pushed"
[[ -z $(git status --porcelain) ]] || fail "the working tree has changes; commit or remove them first"
git fetch --quiet origin main
commit=$(git rev-parse HEAD)
main=$(git rev-parse origin/main)
[[ $commit == "$main" ]] || fail "HEAD is ${commit:0:7}, not origin/main (${main:0:7}); release from an up-to-date main"
echo "${commit:0:7} $(git log -1 --format=%s "$commit")"

step "CI passed for ${commit:0:7}"
ci=$(gh run list --repo "$repo" --workflow ci.yml --commit "$commit" --event push \
	--json status,conclusion --jq 'map(select(.status == "completed")) | .[0].conclusion // "not finished"')
[[ $ci == success ]] || fail "CI for ${commit:0:7} is '$ci'; wait for a green run (gh run list --repo $repo --commit $commit)"
echo "success"

step "Local checks"
just test
just vet
just check
go mod tidy -diff
go run "$govulncheck" ./...

if [[ -n $previous ]]; then
	step "Merged since $previous (material for the notes)"
	git log --first-parent --format='- %s' "$previous..$commit"
fi

if $check_only; then
	step "Every check passed; nothing was changed"
	exit 0
fi

step "Notes"
[[ -s $notes ]] || fail "the notes file '$notes' is missing or empty"
grep -qF "$version" "$notes" || fail "the notes never mention $version (the install line should)"
echo "$notes"

# From here on the release is public and the tag is permanent. If a
# later step fails, say how to finish rather than start again.
finish=""
trap '[[ -n $finish ]] && printf "\nrelease: %s\n" "$finish" >&2' EXIT

step "Tag $version"
run git tag -a "$version" "$commit" -m "htt $version"
run git push origin "refs/tags/$version"
finish="$version is tagged and pushed; publish it with: gh release create $version --repo $repo --title 'htt $version' --notes-file $notes --verify-tag --latest"

step "Publish the GitHub release"
run gh release create "$version" --repo "$repo" --title "htt $version" --notes-file "$notes" --verify-tag --latest
finish="$version is published; check the install by hand: GOBIN=\$(mktemp -d) go install $module/cmd/htt@$version"

if [[ -n $dry_run ]]; then
	step "Dry run: not verifying an install"
	finish=""
	exit 0
fi

step "Verify as a user would"
tmp=$(mktemp -d)
(
	cd "$tmp" # outside any module, as a user would be
	GOBIN="$tmp/bin" go install "$module/cmd/htt@$version"
	got=$("$tmp/bin/htt" --version)
	[[ $got == "htt version $version" ]] || fail "the installed htt says '$got'"
	echo "$got, $(go version -m "$tmp/bin/htt" | head -1 | cut -d' ' -f2)"
	for _ in $(seq 20); do # the proxy's @latest can lag a few minutes
		latest=$(go list -m "$module@latest" | cut -d' ' -f2)
		[[ $latest == "$version" ]] && break
		sleep 15
	done
	[[ $latest == "$version" ]] || fail "$module@latest is still $latest"
	echo "@latest is $latest"
)
rm -rf "$tmp"
tag=$(gh api "repos/$repo/releases/latest" --jq .tag_name)
[[ $tag == "$version" ]] || fail "GitHub's latest release is $tag, not $version"
echo "GitHub's latest release is $tag (what the update check reads)"
finish=""
step "Released $version: https://github.com/$repo/releases/tag/$version"
