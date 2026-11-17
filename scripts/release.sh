#!/usr/bin/env bash
# Mint an htt release: the steps in docs/releasing.md, in order,
# stopping at the first that fails. Run it through just:
#
#   just release-check v0.4.0        # every check; changes nothing
#   just release v0.4.0 notes.md     # check, tag, publish, verify
#   just release-binaries v0.4.0     # rebuild a release's binaries, verify
#
# Publishing a release starts the release-binaries workflow, which
# attaches its binaries; `just release` waits for that run and checks
# what it attached. `just release-binaries` runs the workflow again for
# a release that's already out, and makes the same checks.
#
# RELEASE_DRY_RUN=1 prints the commands that would tag, publish or run
# the workflow instead of running them (and skips verifying the install
# and the binaries).
set -euo pipefail

repo=bricef/htt
module=github.com/bricef/htt
govulncheck=golang.org/x/vuln/cmd/govulncheck@v1.9.0 # the version CI runs
binaries_workflow=release-binaries.yml
here=$(cd "$(dirname "$0")" && pwd)

usage() {
	echo "usage: release.sh --check-only vX.Y.Z | release.sh vX.Y.Z NOTES.md | release.sh --binaries vX.Y.Z" >&2
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

version_of() { # BINARY: what it prints for --version
	# In a scratch home that already has a config file, as dist.sh does:
	# a first run would otherwise create one and, up to v0.3.x, say so on
	# stdout ahead of the version. It also keeps the check out of the
	# real home.
	local home out
	home=$(mktemp -d)
	mkdir -p "$home/.config/htt"
	: >"$home/.config/htt/config.yaml"
	out=$(HOME=$home "$1" --version)
	rm -rf "$home"
	echo "$out"
}

sha256() { # FILE: its SHA-256 digest
	if command -v sha256sum >/dev/null; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

binaries_runs() { # VERSION: "id status conclusion" of its release-binaries runs, newest first
	# The workflow's run-name is "Binaries for <tag>".
	gh run list --repo "$repo" --workflow "$binaries_workflow" --limit 50 \
		--json databaseId,displayTitle,status,conclusion \
		--jq '.[] | [.databaseId, .status, .conclusion, .displayTitle] | @tsv' |
		awk -F '\t' -v title="Binaries for $1" '$4 == title { print $1, $2, $3 }'
}

wait_for_binaries() { # VERSION SEEN: wait for its newest run whose id isn't in SEEN to succeed
	local version=$1 seen=" $2 " newest id status conclusion last="" tries=0
	echo "waiting for the $binaries_workflow run for $version"
	while :; do
		newest=$(binaries_runs "$version" | awk -v seen="$seen" 'index(seen, " " $1 " ") == 0 && !found { print; found = 1 }')
		if [[ -n $newest ]]; then
			read -r id status conclusion <<<"$newest"
			[[ $status == "$last" ]] || echo "run $id: $status"
			last=$status
			[[ $status == completed ]] && break
			((tries < 120)) || fail "run $id is still $status after 30 minutes: https://github.com/$repo/actions/runs/$id"
		else
			((tries < 12)) || fail "no $binaries_workflow run for $version started within 3 minutes: https://github.com/$repo/actions/workflows/$binaries_workflow"
		fi
		tries=$((tries + 1))
		sleep 15
	done
	[[ $conclusion == success ]] || fail "run $id finished '$conclusion': https://github.com/$repo/actions/runs/$id"
	echo "run $id succeeded: https://github.com/$repo/actions/runs/$id"
}

check_binaries() { # VERSION: every expected asset is attached, and this machine's archive checks out
	local version=$1 expected attached name missing="" os arch sums archive tmp want got
	expected=$("$here/dist.sh" --assets "$version")
	attached=$(gh release view "$version" --repo "$repo" --json assets --jq '.assets[].name')
	while read -r name; do
		grep -qxF -- "$name" <<<"$attached" || missing+=" $name"
	done <<<"$expected"
	[[ -z $missing ]] || fail "the $version release is missing:$missing"
	echo "attached: $(paste -sd ' ' - <<<"$expected")"

	os=$(go env GOHOSTOS) arch=$(go env GOHOSTARCH)
	sums=$(grep 'SHA256SUMS$' <<<"$expected")
	archive=$(grep -F "_${os}_$arch." <<<"$expected" || true)
	if [[ -z $archive ]]; then
		echo "there's no archive for this machine ($os/$arch) to try"
		return
	fi
	tmp=$(mktemp -d)
	gh release download "$version" --repo "$repo" --dir "$tmp" --pattern "$archive" --pattern "$sums"
	want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/$sums")
	got=$(sha256 "$tmp/$archive")
	[[ -n $want && $got == "$want" ]] || fail "$archive's SHA-256 is $got, but $sums says '$want'"
	echo "$archive matches $sums"
	gh attestation verify "$tmp/$archive" --repo "$repo" --signer-workflow "$repo/.github/workflows/$binaries_workflow"
	echo "$archive's provenance attestation verifies"
	tar -xzf "$tmp/$archive" -C "$tmp" htt
	got=$(version_of "$tmp/htt")
	[[ $got == "htt version $version" ]] || fail "the downloaded htt says '$got'"
	echo "the downloaded htt says: $got"
	rm -rf "$tmp"
}

check_only=false
binaries_only=false
case ${1:-} in
--check-only)
	check_only=true
	shift
	;;
--binaries)
	binaries_only=true
	shift
	;;
esac
version=${1:-}
notes=${2:-}
[[ -n $version ]] || usage
$check_only || $binaries_only || [[ -n $notes ]] || usage

if $binaries_only; then
	step "Rebuild the binaries for the $version release"
	"$here/dist.sh" --assets "$version" >/dev/null # a version, not some other ref
	gh release view "$version" --repo "$repo" --json url --jq .url
	seen=$(binaries_runs "$version" | cut -d' ' -f1 | tr '\n' ' ')
	run gh workflow run "$binaries_workflow" --repo "$repo" -f "tag=$version"
	if [[ -n $dry_run ]]; then
		step "Dry run: not waiting for or checking the binaries"
		exit 0
	fi
	wait_for_binaries "$version" "$seen"
	step "Check the binaries"
	check_binaries "$version"
	step "$version's binaries are attached: https://github.com/$repo/releases/tag/$version"
	exit 0
fi

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
finish="$version is published; check the install by hand: GOBIN=\$(mktemp -d) go install $module/cmd/htt@$version; and the binaries with: just release-binaries $version"

if [[ -n $dry_run ]]; then
	step "Dry run: not verifying the install or the binaries"
	echo "the $binaries_workflow run would attach: $("$here/dist.sh" --assets "$version" | paste -sd ' ' -)"
	finish=""
	exit 0
fi

step "Verify as a user would"
tmp=$(mktemp -d)
(
	cd "$tmp" # outside any module, as a user would be
	GOBIN="$tmp/bin" go install "$module/cmd/htt@$version"
	got=$(version_of "$tmp/bin/htt")
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
finish="$version is released and go install works, but its binaries aren't checked; once the cause is fixed, rebuild and check them with: just release-binaries $version"

step "Binaries from the $binaries_workflow run"
wait_for_binaries "$version" ""
check_binaries "$version"
finish=""
step "Released $version: https://github.com/$repo/releases/tag/$version"
