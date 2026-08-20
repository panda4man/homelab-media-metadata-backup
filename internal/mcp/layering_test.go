package mcp

import (
	"os/exec"
	"strings"
	"testing"
)

// A stray httpapi or apiclient import here would pull in internal/orchestrator
// transitively, which is exactly what this test guards against.
func TestPackageLayering_DoesNotDependOnHTTPAPIOrOrchestrator(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps . failed: %v\n%s", err, out)
	}

	forbidden := []string{
		"homelab-media-metadata-backup/internal/httpapi",
		"homelab-media-metadata-backup/internal/apiclient",
		"homelab-media-metadata-backup/internal/orchestrator",
	}
	deps := string(out)
	for _, pkg := range forbidden {
		if strings.Contains(deps, pkg) {
			t.Errorf("internal/mcp depends on %s, want no such dependency", pkg)
		}
	}
}
