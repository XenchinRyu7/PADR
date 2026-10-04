package scheduler_test

import (
	"context"
	"testing"

	"github.com/padr-runner/padr/pkg/scheduler"
)

func TestSchedulerInstantiation(t *testing.T) {
	s := scheduler.NewScheduler()
	if s == nil {
		t.Fatalf("expected non-nil scheduler")
	}

	// Just check if List returns without panicking
	_, _ = s.List(context.Background())
}
