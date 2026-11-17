# Releasing

A release is an annotated semver tag on `main` plus a GitHub Release.
The update check announces releases (it reads
`/repos/bricef/htt/releases/latest`), and `go install …@latest`
resolves to the newest tag, so both matter.

Releases are made by hand, when you decide to, with two `just`
recipes (both run `scripts/release.sh`). You need `gh` signed in to
GitHub and an up-to-date checkout of `main`.

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

   It ends by listing what was merged since the last release, to write
   the notes from. Fix anything govulncheck says htt's code reaches
   before releasing; a standard-library fix means raising the `go`
   directive in `go.mod` (see [Go versions](#go-versions)).

3. **Write the notes** in a Markdown file outside the repo: what's new,
   how to install (`go install github.com/bricef/htt/cmd/htt@vX.Y.Z`),
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
     catches up), and GitHub's latest release.

   If a step fails after the tag is pushed, the recipe prints the
   command that finishes the release; don't start again (see below).
   `RELEASE_DRY_RUN=1 just release …` prints the tag and publish
   commands instead of running them.

5. **Check the update notice** if you like: an older build run at a
   terminal should now print it.

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
