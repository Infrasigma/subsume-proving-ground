package ace

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func testProvenanceParent(t *testing.T) string {
	t.Helper()
	run := func(args ...string) string {
		b, err := exec.Command("git", args...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	parents := strings.Fields(run("show", "-s", "--format=%P", "HEAD"))
	if len(parents) > 0 {
		return parents[0]
	}
	if base := strings.TrimSpace(os.Getenv("GITHUB_BASE_REF")); base != "" {
		if parent := run("rev-parse", "origin/"+base); parent != "" {
			return parent
		}
	}
	if before := strings.TrimSpace(os.Getenv("GITHUB_EVENT_BEFORE")); before != "" && before != strings.Repeat("0", 40) {
		return before
	}
	t.Fatalf("provenance parent unavailable")
	return ""
}
