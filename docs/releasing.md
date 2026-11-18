# Releasing

A release is an annotated semver tag on `main` plus a GitHub Release,
with binaries attached. The update check announces releases (it reads
`/repos/bricef/htt/releases/latest`), and `go install …@latest`
resolves to the newest tag, so both matter; the binaries are for
people without Go.

Releases are made by hand, when you decide to, with `just` recipes
that run `scripts/release.sh`. You need `gh` signed in to GitHub and
an up-to-date checkout of `main`. Publishing the release starts a
GitHub Actions workflow that builds the binaries (see
[Binaries](#binaries)).

## Steps

1. **Pick the version.** Semver. While htt is `v0.x`, a release that
   adds features or removes one bumps the minor version, and a
   fixes-only release bumps the patch.

2. **Check `main`:**

   ```
   just release-check vX.Y.Z
   ```

   This changes nothing. It stops at the first check that fails:

   - the version is `vX.Y.Z`, not yet tagged, and newer than the last
     release;
   - the working tree is clean and `HEAD` is `origin/main`;
   - CI passed for that commit;
   - `just test`, `just vet`, `just check`, `go mod tidy -diff` and
     govulncheck (the version CI runs) pass locally.

   It also looks up whether the weekly vulnerability scan
   (`weekly.yml`) is still on: GitHub turns a scheduled workflow off
   after 60 days without activity and never turns it back on. If it's
   off, it warns with the command that turns it back on
   (`gh workflow enable weekly.yml --repo bricef/htt`); if the lookup
   fails, it says so. Neither stops a release, and `just release`
   checks the same.

   It ends by listing what was merged since the last release, to write
   the notes from. Fix anything govulncheck says htt's code reaches
   before releasing; a standard-library fix means raising the `go`
   directive in `go.mod` (see [Go versions](#go-versions)).

3. **Write the notes** in a Markdown file outside the repo: what's new,
   how to install (`go install github.com/bricef/htt/cmd/htt@vX.Y.Z`,
   or download a binary, as the README's Installation section says),
   and where the known issues are (`TODO.md`). The recipe refuses notes
   that don't mention the version.

4. **Release:**

   ```
   just release vX.Y.Z /path/to/notes.md
   ```

   It runs every check again, then:

   - tags the commit (`git tag -a vX.Y.Z <commit> -m "htt vX.Y.Z"`)
     and pushes the tag;
   - publishes the GitHub Release (`gh release create … --verify-tag
     --latest`): not a draft and not a pre-release, since
     `/releases/latest` skips both and the update check wouldn't see
     it;
   - verifies it as a user would, from a temporary directory outside
     the repo: `go install …@vX.Y.Z`, `htt --version`, `go list -m
     …@latest` (retrying for a few minutes while the module proxy
     catches up), and GitHub's latest release;
   - waits for the release-binaries run for the tag (up to 3 minutes
     for it to start and 30 to finish), then checks the release: every
     archive and the checksums file are attached, and the archive for
     this machine matches the checksums, has an attestation from the
     workflow (`gh attestation verify`), and holds an `htt` whose
     `--version` is `htt version vX.Y.Z`.

   If a step fails after the tag is pushed, the recipe prints the
   command that finishes the release; don't start again (see below).
   For the binaries that's `just release-binaries vX.Y.Z`, once
   whatever failed is fixed. `RELEASE_DRY_RUN=1 just release …` prints
   the tag and publish commands instead of running them, and lists the
   files the binaries run would attach.

5. **Check the update notice** if you like: an older build run at a
   terminal should now print it.

## Binaries

[`release-binaries.yml`](../.github/workflows/release-binaries.yml)
runs when a release is published. It builds, for Linux and macOS on
amd64 and arm64:

- `htt_vX.Y.Z_<os>_<arch>.tar.gz`, holding `htt`, `README.md` and
  `LICENSE` (releases before the MIT licence was added, v0.3.0 and
  earlier, have no `LICENSE`);
- `htt_vX.Y.Z_SHA256SUMS`, the SHA-256 of each archive;

then makes a signed build-provenance attestation for the archives and
uploads them all to the release, replacing any of the same name. The
build and the upload are separate jobs, so the build, which compiles
third-party code, holds no token that can write to the repository or
sign attestations.

The build is [`scripts/dist.sh`](../scripts/dist.sh), and `just dist
vX.Y.Z` runs it locally, into `dist/`. It builds the tagged commit,
whatever is checked out, from a fresh clone of the tag
(`CGO_ENABLED=0`, `-trimpath`, stripped). The version matters:
`htt --version` prints the one Go stamped into the binary, and the
update check is off in a build without one. Go stamps the tag's
version only when the build runs in a clean git checkout with the tag
at `HEAD`, which the clone is. The script checks every binary's build
info (`go version -m`: the version, platform, `CGO_ENABLED=0`,
`-trimpath`, no local modifications), runs the one for the machine it
runs on, and fails if anything is off.

`just release-binaries vX.Y.Z` runs the workflow again for a release
that's already published (`gh workflow run release-binaries.yml -f
tag=vX.Y.Z`, from `main`), waits for that run, and makes the same
checks as `just release`. Use it when a release's binaries failed, or
to give an older release binaries. The workflow has to be on `main`
for this to work.

There's no Windows build: htt doesn't build for Windows, because
goterm v1.0.3 uses `unix.EOPNOTSUPP` on every platform (goterm
v1.0.4 fixes it). Once htt builds there, adding `windows/amd64` and
`windows/arm64` to the platforms in `scripts/dist.sh` is all it takes;
their archives are zips.

## Tags are permanent

The Go module proxy caches every version it has served, forever. Never
move, delete or reuse a tag; if a release is wrong, fix forward with a
new patch release. A GitHub Release's notes can be edited afterwards
(`gh release edit vX.Y.Z --notes-file notes.md`).

## Go versions

`go install pkg@version` builds with the user's own Go, switching to a
newer toolchain only when the module's `go` directive asks for one. It
ignores the `toolchain` line, which only applies to builds inside a
checkout. So to make every build use a patched standard library, raise
the `go` directive (`go get go@1.26.N`); with the default
`GOTOOLCHAIN=auto`, an older local Go downloads that version for the
build. v0.1.0 learned this the hard way: it pinned the toolchain line
only, and `go install` built it with whatever Go the user had.
