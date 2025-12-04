package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
)

func TestDefaultData(t *testing.T) {
	if got := shared.DefaultData("", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if got := shared.DefaultData("value", "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}

func TestRun_Version(t *testing.T) {
	Version = "1.2.3"
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"ver"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.String() != "Version : 1.2.3\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_VersionUsesBuildInfo(t *testing.T) {
	origVersion := Version
	Version = defaultVersion
	t.Cleanup(func() { Version = origVersion })

	origReader := buildInfoReader
	buildInfoReader = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v9.9.9"},
		}, true
	}
	t.Cleanup(func() { buildInfoReader = origReader })

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"ver"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.String() != "Version : v9.9.9\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "run 'ppkgmgr help'") {
		t.Fatalf("expected help suggestion, got %q", stderr.String())
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"help"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "Available Commands") {
		t.Fatalf("expected help text, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
