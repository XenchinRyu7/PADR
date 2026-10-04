package cmd_test

import (
	"os"
	"testing"

	"github.com/padr-runner/padr/cmd/padr/cmd"
)

func TestRootCommandExecution(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "padr-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)
	t.Setenv("PADR_HOME", tempHome)

	// Test init command
	os.Args = []string{"padr", "init"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("padr init failed: %v", err)
	}

	// Test models command
	os.Args = []string{"padr", "models"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("padr models failed: %v", err)
	}

	// Test quota command
	os.Args = []string{"padr", "quota"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("padr quota failed: %v", err)
	}

	// Test status command
	os.Args = []string{"padr", "status"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("padr status failed: %v", err)
	}
}
