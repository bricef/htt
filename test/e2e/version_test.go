package e2e

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// versionVar is the variable a build sets to give htt its version.
const versionVar = "github.com/bricef/htt/internal/update.version"

// buildHtt builds htt the way a packager building a release's source
// archive would: with no git information (-buildvcs=false, as from a
// tarball), plus any extra build flags.
func buildHtt(t *testing.T, flags ...string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	bin := filepath.Join(t.TempDir(), exe("htt"))
	args := append([]string{"build", "-buildvcs=false", "-o", bin}, flags...)
	cmd := exec.Command("go", append(args, "./cmd/htt")...)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %v: %v\n%s", flags, err, out)
	}
	return bin
}

func versionOf(t *testing.T, bin string) string {
	t.Helper()
	e := newEnv(t) // a home with a config file, so only the version prints
	cmd := exec.Command(bin, "--version")
	cmd.Env = append(cmd.Environ(), e.homeEnv()...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s --version: %v", bin, err)
	}
	return strings.TrimSpace(string(out))
}

func TestVersion_SetAtBuildTimeForASourceBuild(t *testing.T) {
	// Without git information Go records no version, so a build from a
	// source archive needs one passed in. A version without its v, as
	// package managers write it, works too.
	for _, given := range []string{"v0.9.9", "0.9.9"} {
		bin := buildHtt(t, "-ldflags", "-X "+versionVar+"="+given)
		if got := versionOf(t, bin); got != "htt version v0.9.9" {
			t.Errorf("built with version %s: --version says %q, want %q", given, got, "htt version v0.9.9")
		}
	}
}

func TestVersion_UnknownForASourceBuildWithoutOne(t *testing.T) {
	bin := buildHtt(t)
	if got := versionOf(t, bin); got != "htt version unknown (development build)" {
		t.Errorf("--version says %q", got)
	}
}
