#!/usr/bin/env bash
# Build htt's release archives: a binary for each platform, packed with
# README.md (and LICENSE, if the tagged commit has one) and, under
# third-party/, the licences of the third-party code and fonts the GUI
# embeds and of Go and the Go modules linked into that binary, plus a
# SHA256SUMS file. The release-binaries workflow runs this for every
# published release and uploads the result. Locally:
#
#   just dist v0.4.0                   # into dist/
#   scripts/dist.sh VERSION OUTDIR
#   scripts/dist.sh --assets VERSION   # the file names a release carries
#   scripts/dist.sh --check-go-licences ARCHIVE  # check its third-party/go/
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
unix_platforms="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"
# A Windows archive is a zip holding htt.exe, as Go's own are; the
# others are tarballs holding htt. Releases up to v0.4.0 have no Windows
# build: they use goterm v1.0.3, which doesn't build there.
windows_platforms="windows/amd64 windows/arm64"
last_without_windows=v0.4.0
# Modules a binary links that have no licence file at their root, and
# the file there that states their licence instead, a line each:
# MODULE@VERSION FILE NOTE (for the index). Each is checked by hand.
# go-localereader is linked on Windows only (through bubbletea); its
# only tag predates its LICENSE file.
licence_elsewhere='github.com/mattn/go-localereader@v0.0.1 README.md this version has no licence file; its README says MIT'

usage() {
	echo "usage: dist.sh VERSION OUTDIR | dist.sh --assets VERSION | dist.sh --check-go-licences ARCHIVE" >&2
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
platforms_of() { # VERSION: the platforms its release is built for
	if [[ $(printf '%s\n%s\n' "$last_without_windows" "$1" | sort -V | tail -1) == "$last_without_windows" ]]; then
		echo "$unix_platforms"
	else
		echo "$unix_platforms $windows_platforms"
	fi
}
assets() { # VERSION: every file a release carries, archives first
	local p
	for p in $(platforms_of "$1"); do
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
linked_modules() { # BINARY: "path version" of each module it links, as its build info lists them
	local mods
	mods=$(go version -m "$1" | awk '$1 == "dep" { print $2, $3 }')
	# Every htt links some, so none means the build info wasn't read.
	[[ -n $mods ]] || fail "go version -m lists no modules for $1"
	echo "$mods"
}
case_encode() { # STRING: as the module cache spells it, each capital letter as ! and its lower case
	LC_ALL=C awk '{
		s = ""
		for (i = 1; i <= length($0); i++) {
			c = substr($0, i, 1)
			s = s (c ~ /[A-Z]/ ? "!" tolower(c) : c)
		}
		print s
	}' <<<"$1"
}
collect_go_licences() { # BINARY STAGE PLATFORM: copy the licences of Go and of each module BINARY links into STAGE/third-party/go, with an index, and list the files in gofiles
	local bin=$1 stage=$2 platform=$3 info built mods gotree path mversion from found note file names rows
	gotree=$stage/third-party/go
	info=$(go version -m "$bin")
	built=$(head -1 <<<"$info" | awk '{ print $NF }')
	[[ $built == "$goversion" ]] || fail "the $platform binary was built with $built, not $goversion, whose licence this would ship"
	# A replaced module's code, and so its licence, come from elsewhere.
	if grep -q '^[[:space:]]*=>' <<<"$info"; then
		fail "the $platform binary links a replaced module (=> in go version -m), and dist.sh doesn't look up a replacement's licence"
	fi
	mods=$(linked_modules "$bin")

	mkdir -p "$gotree/std"
	cp "$goroot/LICENSE" "$gotree/std/LICENSE"
	chmod 0644 "$gotree/std/LICENSE"
	gofiles=(third-party/go/std/LICENSE)
	rows="| std (Go's runtime and standard library) | $goversion | \`LICENSE\` |"
	while read -r path mversion; do
		from=$gomodcache/$(case_encode "$path")@$(case_encode "$mversion")
		[[ -d $from ]] || fail "$path $mversion, which the $platform binary links, isn't in the module cache ($from)"
		# Not Go files: a license.go or copying.go is code.
		found=$(find "$from" -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'LICENCE*' -o -iname 'COPYING*' -o -iname 'NOTICE*' \) ! -iname '*.go' | sed 's|.*/||' | LC_ALL=C sort)
		note=""
		if [[ -z $found ]]; then
			found=$(awk -v m="$path@$mversion" '$1 == m { print $2 }' <<<"$licence_elsewhere")
			[[ -n $found && -f $from/$found ]] ||
				fail "$path $mversion, which the $platform binary links, has no licence file (LICENSE*, LICENCE*, COPYING* or NOTICE*) at its root"
			note=" ($(awk -v m="$path@$mversion" '$1 == m { $1 = $2 = ""; sub(/^ +/, ""); print }' <<<"$licence_elsewhere"))"
		fi
		mkdir -p "$gotree/$path"
		names=""
		while IFS= read -r file; do
			cp "$from/$file" "$gotree/$path/$file"
			chmod 0644 "$gotree/$path/$file"
			gofiles+=("third-party/go/$path/$file")
			names+="${names:+, }\`$file\`"
		done <<<"$found"
		rows+=$'\n'"| $path | $mversion | $names$note |"
	done <<<"$mods"

	{
		echo "# Go and Go modules"
		echo
		echo "The htt in this archive ($version for $platform) is written in Go"
		echo "and built with $goversion. It links in Go's runtime and standard"
		echo "library, and the Go modules below: the ones \`go version -m\` lists"
		echo "for it. Their licences ask for their notices to go with copies of"
		echo "the binary, so they are here: Go's licence in \`std/\`, and each"
		echo "module's licence and notice files, from the root of the version"
		echo "linked, in a directory named after the module's path."
		echo
		echo "The source of each, at the version listed, is published: Go's at"
		echo "https://go.dev/dl/, and a module's on the Go module mirror, which"
		echo "\`go mod download MODULE@VERSION\` fetches it from"
		echo "(https://proxy.golang.org/MODULE/@v/VERSION.zip, with each capital"
		echo "letter in MODULE written as ! and its lower case)."
		echo
		echo "| Module | Version | Licence files |"
		echo "|---|---|---|"
		echo "$rows"
	} >"$gotree/README.md"
	chmod 0644 "$gotree/README.md"
	gofiles+=(third-party/go/README.md)
}
check_go_licences() { # ARCHIVE DIR: unpack ARCHIVE into DIR, check its third-party/go against its htt, and set linked to the number of modules it links
	# Go's licence, the index, and for every module the binary links
	# (go version -m), a row in the index for that version naming at
	# least one file, and each file it names. It reads the binary, not the
	# index, for the list, so a module the packing missed shows up.
	local archive=$1 dir=$2 exe mods gotree missing="" path mversion row files file
	case $archive in
	*.zip)
		exe=htt.exe
		unzip -q "$archive" -d "$dir"
		;;
	*.tar.gz)
		exe=htt
		tar -xzf "$archive" -C "$dir"
		;;
	*) fail "$archive isn't a release archive (.tar.gz or .zip)" ;;
	esac
	[[ -f $dir/$exe ]] || fail "$archive holds no $exe"
	mods=$(linked_modules "$dir/$exe")
	gotree=$dir/third-party/go
	[[ -d $gotree ]] || fail "$(basename "$archive") has no third-party/go/"
	[[ -s $gotree/README.md ]] || fail "$(basename "$archive") has no index, third-party/go/README.md"
	[[ -s $gotree/std/LICENSE ]] || missing+=" Go's licence (std/LICENSE);"
	linked=0
	while read -r path mversion; do
		linked=$((linked + 1))
		row=$(awk -v row="| $path | $mversion | " 'index($0, row) == 1 { print substr($0, length(row) + 1); exit }' "$gotree/README.md")
		if [[ -z $row ]]; then
			missing+=" $path $mversion in the index;"
			continue
		fi
		files=$(awk '{ # the `quoted` names
			s = $0
			while (match(s, /`[^`]+`/)) {
				print substr(s, RSTART + 1, RLENGTH - 2)
				s = substr(s, RSTART + RLENGTH)
			}
		}' <<<"$row")
		[[ -n $files ]] || missing+=" a licence file for $path in the index;"
		while IFS= read -r file; do
			[[ -z $file || -s $gotree/$path/$file ]] || missing+=" $path/$file;"
		done <<<"$files"
	done <<<"$mods"
	[[ -z $missing ]] || fail "$(basename "$archive")'s third-party/go/ lacks:${missing%;}"
}

case ${1:-} in
--assets)
	[[ $# == 2 ]] || usage
	check_version "$2"
	assets "$2"
	exit 0
	;;
--check-go-licences)
	[[ $# == 2 ]] || usage
	[[ -f $2 ]] || fail "there's no archive $2"
	scratch=$(mktemp -d)
	trap 'rm -rf "$scratch"' EXIT
	check_go_licences "$2" "$scratch"
	echo "$(basename "$2"): third-party/go/ has Go's licence and those of the $linked modules its htt links"
	exit 0
	;;
esac
[[ $# == 2 ]] || usage
version=$1
check_version "$version"
platforms=$(platforms_of "$version")
if [[ " $platforms" == *" windows/"* ]]; then
	command -v zip >/dev/null || fail "the Windows archives need zip; install it (apt install zip)"
	command -v unzip >/dev/null || fail "checking the Windows archives needs unzip; install it (apt install unzip)"
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
# The GUI embeds third-party code and fonts (ECharts, IBM Plex) from
# internal/gui/web/vendor. Their licences ask for a copy to go with every
# copy of the software, so each archive carries every licence, notice and
# README there under third-party/, keeping their paths. Releases before
# the GUI have no vendor directory, and nothing changes for them.
vendor=internal/gui/web/vendor
third=()
if [[ -d $src/$vendor ]]; then
	while IFS= read -r file; do
		third+=("third-party/${file#"$src/$vendor/"}")
	done < <(find "$src/$vendor" -type f \( -iname 'LICENSE*' -o -iname 'NOTICE*' -o -iname 'README*' \) | LC_ALL=C sort)
	[[ ${#third[@]} -gt 0 ]] || fail "$version vendors third-party files under $vendor but has no licence for them"
	[[ ! -e $src/$vendor/go ]] || fail "$vendor/go would land on third-party/go/, where the Go licences go"
fi
# Each binary also links in Go's runtime and standard library and the Go
# modules its build info lists (go version -m; the list differs by
# platform). Their licences, mostly MIT, BSD and Apache-2.0, ask for
# their notices to go with copies of the binary, so each archive
# carries, under third-party/go/, Go's LICENSE as std/LICENSE, from the
# Go that built it, and every licence and notice file at the root of each
# module the binary links, under the module's path, from the module
# cache the build used; and an index, third-party/go/README.md. A module
# with no licence file fails the build, unless it's in
# licence_elsewhere. This holds for every version, so an older release
# rebuilt gains these files and nothing else changes.
clone_env() { (cd "$src" && go env "$1"); } # as the build sees it: go.mod's go line can switch the toolchain
goversion=$(clone_env GOVERSION)
goroot=$(clone_env GOROOT)
gomodcache=$(clone_env GOMODCACHE)
[[ -f $goroot/LICENSE ]] || fail "$goversion has no LICENSE in $goroot"
gofiles=()

host="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
ran=""
archives=()
echo "Building htt $version ($(git -C "$src" rev-parse --short HEAD)) with $goversion"
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
	for doc in ${third[@]+"${third[@]}"}; do # the guard keeps bash 3's set -u happy when empty
		mkdir -p "$stage/$(dirname "$doc")"
		cp "$src/$vendor/${doc#third-party/}" "$stage/$doc"
		chmod 0644 "$stage/$doc"
	done
	collect_go_licences "$stage/$exe" "$stage" "$platform"
	chmod 0755 "$stage/$exe"
	name=$(archive_name "$version" "$os" "$arch")
	rm -f "$out/$name"
	if [[ $os == windows ]]; then
		(cd "$stage" && zip -q -X "$out/$name" "$exe" "${docs[@]}" ${third[@]+"${third[@]}"} "${gofiles[@]}")
	else
		(cd "$stage" && tar -cf - "$exe" "${docs[@]}" ${third[@]+"${third[@]}"} "${gofiles[@]}" | gzip -9n >"$out/$name")
	fi
	mkdir "$work/check-$os-$arch"
	check_go_licences "$out/$name" "$work/check-$os-$arch"
	archives+=("$name")
	echo "  $name, with the licences of Go and the $linked modules $exe links"
done

sums=$(sums_name "$version")
(cd "$out" && sha256 "${archives[@]}") >"$out/$sums"
echo "  $sums"
echo "${ran:-No binary built for this machine ($host), so none was run}"
echo "Every binary is stamped $version; the archives are in $out"
