#!/usr/bin/env bash
# Build htt's release archives: a binary for each platform, packed with
# README.md (and LICENSE, if the tagged commit has one), plus a
# SHA256SUMS file. The release-binaries workflow runs this for every
# published release and uploads the result. Locally:
#
#   just dist v0.4.0                   # into dist/
#   scripts/dist.sh VERSION OUTDIR
#   scripts/dist.sh --assets VERSION   # the file names a release carries
#
# It builds the commit tagged VERSION in this repository, whatever is
# checked out, from a fresh clone of the tag. Go stamps a binary built
# in a clean git checkout of a tag with that version, and the version is
# what `htt --version` prints and what the update check compares. A
# build outside such a checkout gets a pseudo-version, a +dirty suffix,
# or no version at all ("(devel)"), which also switches the update check
# off. So every binary's stamp is checked, and the one this machine can
# run is run.
set -euo pipefail

module=github.com/bricef/htt
# Windows is missing because htt doesn't build there yet: goterm v1.0.3
# uses unix.EOPNOTSUPP on every platform (fixed in goterm v1.0.4). Once
# it builds, add windows/amd64 and windows/arm64; their archives are
# zips.
platforms="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"

usage() {
	echo "usage: dist.sh VERSION OUTDIR | dist.sh --assets VERSION" >&2
	exit 2
}
fail() {
	printf 'dist: %s\n' "$*" >&2
	exit 1
}
check_version() {
	[[ $1 =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] ||
		fail "$1 isn't a version tag like v1.2.3"
}
archive_name() { # VERSION OS ARCH
	local ext=tar.gz
	[[ $2 == windows ]] && ext=zip
	echo "htt_$1_$2_$3.$ext"
}
sums_name() { # VERSION
	echo "htt_$1_SHA256SUMS"
}
assets() { # VERSION: every file a release carries, archives first
	local p
	for p in $platforms; do
		archive_name "$1" "${p%/*}" "${p#*/}"
	done
	sums_name "$1"
}
version_of() { # BINARY: what it prints for --version
	# Run in a scratch home that already has a config file: on a first
	# run htt creates one and, up to v0.3.x, says so on stdout, ahead of
	# the version (a fresh CI runner is a first run). Nothing touches the
	# real home either.
	local home out
	home=$(mktemp -d)
	mkdir -p "$home/.config/htt"
	: >"$home/.config/htt/config.yaml"
	out=$(HOME=$home "$1" --version)
	rm -rf "$home"
	echo "$out"
}
sha256() { # FILE...: "digest  name" lines, as sha256sum prints them
	if command -v sha256sum >/dev/null; then
		sha256sum "$@"
	else
		shasum -a 256 "$@"
	fi
}

if [[ ${1:-} == --assets ]]; then
	[[ $# == 2 ]] || usage
	check_version "$2"
	assets "$2"
	exit 0
fi
[[ $# == 2 ]] || usage
version=$1
check_version "$version"
if [[ " $platforms" == *" windows/"* ]] && ! command -v zip >/dev/null; then
	fail "the Windows archives need zip; install it (apt install zip)"
fi

repo=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
git -C "$repo" rev-parse --quiet --verify "refs/tags/$version^{commit}" >/dev/null ||
	fail "there's no tag $version in $repo (git fetch --tags?)"
mkdir -p "$2"
out=$(cd "$2" && pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
src=$work/src
# A clone has the tag, the .git data Go reads it from, and no untracked
# files to mark the build +dirty.
git -c advice.detachedHead=false clone --quiet --branch "$version" "$repo" "$src"
[[ -f $src/README.md ]] || fail "$version has no README.md"
docs=(README.md)
if [[ -f $src/LICENSE ]]; then
	docs+=(LICENSE)
else
	echo "note: $version has no LICENSE file, so the archives hold the binary and README.md"
fi

host="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
ran=""
archives=()
echo "Building htt $version ($(git -C "$src" rev-parse --short HEAD)) with $(go env GOVERSION)"
for platform in $platforms; do
	os=${platform%/*} arch=${platform#*/}
	exe=htt
	[[ $os == windows ]] && exe=htt.exe
	stage=$work/$os-$arch
	mkdir "$stage"
	(cd "$src" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
		go build -trimpath -buildvcs=true -ldflags='-s -w' -o "$stage/$exe" ./cmd/htt)

	info=$(go version -m "$stage/$exe")
	stamp=$(awk -v m="$module" '$1 == "mod" && $2 == m { print $3 }' <<<"$info")
	[[ $stamp == "$version" ]] || fail "the $platform binary is stamped '$stamp', not $version"
	settings=$(awk '$1 == "build" { print $2 }' <<<"$info")
	for want in "GOOS=$os" "GOARCH=$arch" CGO_ENABLED=0 -trimpath=true vcs.modified=false; do
		grep -qxF -- "$want" <<<"$settings" || fail "the $platform binary wasn't built with $want"
	done
	if [[ $platform == "$host" ]]; then
		says=$(version_of "$stage/$exe")
		[[ $says == "htt version $version" ]] || fail "the $platform binary says '$says'"
		ran="$platform binary says: $says"
	fi

	for doc in "${docs[@]}"; do
		cp "$src/$doc" "$stage/$doc"
		chmod 0644 "$stage/$doc"
	done
	chmod 0755 "$stage/$exe"
	name=$(archive_name "$version" "$os" "$arch")
	rm -f "$out/$name"
	if [[ $os == windows ]]; then
		(cd "$stage" && zip -q -X "$out/$name" "$exe" "${docs[@]}")
	else
		(cd "$stage" && tar -cf - "$exe" "${docs[@]}" | gzip -9n >"$out/$name")
	fi
	archives+=("$name")
	echo "  $name"
done

sums=$(sums_name "$version")
(cd "$out" && sha256 "${archives[@]}") >"$out/$sums"
echo "  $sums"
echo "${ran:-No binary built for this machine ($host), so none was run}"
echo "Every binary is stamped $version; the archives are in $out"
