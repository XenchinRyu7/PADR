package github_test

import (
	"context"
	"testing"

	"github.com/padr-runner/padr/pkg/github"
)

func TestDetectAccount(t *testing.T) {
	acc := github.DetectAccount(context.Background())
	if acc == nil {
		t.Fatalf("expected non-nil account info")
	}
	// On this machine, XenchinRyu7 is configured
	if acc.Username == "" {
		t.Logf("no username detected, which is acceptable in bare environments")
	} else {
		t.Logf("Detected GitHub User: %s, Email: %s, Method: %s", acc.Username, acc.Email, acc.Method)
	}
}
