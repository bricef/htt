# Releasing

A release is an annotated semver tag on `main` plus a GitHub Release.
The update check announces releases (it reads
`/repos/bricef/htt/releases/latest`), and `go install …@latest`
resolves to the newest tag, so both matter.

## Steps

1. **Check `main`.** From an up-to-date checkout of `main`:

   ```
   just test && just vet && just check
   go run golang.org/x/vuln/cmd/govulncheck@latest ./...
   ```

   Fix anything govulncheck says htt's code reaches before tagging. A
   standard-library fix means raising the `go` directive in `go.mod`
   (see [Go versions](#go-versions)).

2. **Pick the version.** Semver. While htt is `v0.x`, a release with
   new features bumps the minor version and a fixes-only release bumps
   the patch.

3. **Tag the commit** with an annotated tag and push it:

   ```
   git tag -a vX.Y.Z <commit> -m "htt vX.Y.Z"
   git push origin refs/tags/vX.Y.Z
   ```

4. **Publish the GitHub Release** from that tag, with notes:

   ```
   gh release create vX.Y.Z --title "htt vX.Y.Z" --notes-file notes.md --verify-tag --latest
   ```

   Not a draft and not a pre-release: `/releases/latest` skips both,
   so the update check wouldn't see it. Notes say what's new, how to
   install, and point at known issues (`TODO.md`, open issues).

5. **Verify it as a user would**, from a directory outside the repo:

   ```
   GOBIN=$(mktemp -d) go install github.com/bricef/htt/cmd/htt@vX.Y.Z
   $GOBIN/htt --version                       # htt version vX.Y.Z
   go version -m $GOBIN/htt | head -1         # built with the Go you expect
   go list -m github.com/bricef/htt@latest    # vX.Y.Z
   ```

   An older build run at a terminal should now print the update
   notice.

## Tags are permanent

The Go module proxy caches every version it has served, forever. Never
move, delete or reuse a tag; if a release is wrong, fix forward with a
new patch release. A GitHub Release's notes can be edited afterwards.

## Go versions

`go install pkg@version` builds with the user's own Go, switching to a
newer toolchain only when the module's `go` directive asks for one. It
ignores the `toolchain` line, which only applies to builds inside a
checkout. So to make every build use a patched standard library, raise
the `go` directive (`go get go@1.26.N`); with the default
`GOTOOLCHAIN=auto`, an older local Go downloads that version for the
build. v0.1.0 learned this the hard way: it pinned the toolchain line
only, and `go install` built it with whatever Go the user had.
