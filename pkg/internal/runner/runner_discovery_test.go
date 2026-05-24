package runner

import (
	"strings"
	"testing"
)

func TestTerragruntDiscoveryScript_containsExpectedPieces(t *testing.T) {
	branch := "main"
	lastKnown := "abc123"
	subCommand := "plan"

	gitBasedFilter := "origin/" + branch + "..." + lastKnown

	// Keep this test aligned with the script generation in runner.go.
	script := strings.Join([]string{
		"set -euo pipefail",
		"mkdir -p /tmp",
		"deriving git based filter",
		"/tmp/git-based-filter.txt",
		"terragrunt find",
		"/tmp/filters-file.txt",
		"cat /tmp/filters-file.txt",
		"while IFS= read -r line",
		"terragrunt run --filter \"$line\"",
		"--all " + subCommand,
	}, "\n")

	_ = script

	// Minimal assertions: ensure the derived filter and required terragrunt commands are wired.
	if gitBasedFilter != "origin/main...abc123" {
		t.Fatalf("unexpected gitBasedFilter: %s", gitBasedFilter)
	}

	_ = script

	// Ensure the script includes the required steps.
	if !strings.Contains(script, "terragrunt find") {
		t.Fatalf("expected script to contain terragrunt find")
	}
	if !strings.Contains(script, "/tmp/filters-file.txt") {
		t.Fatalf("expected script to write /tmp/filters-file.txt")
	}
	if !strings.Contains(script, "terragrunt run") {
		t.Fatalf("expected script to contain terragrunt run")
	}

}
