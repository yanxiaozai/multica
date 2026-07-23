package scheduler

import (
	"context"
	"errors"
	"testing"
)

// fakeIssueSyncRunner is a minimal IssueSyncRunner that records the call and
// returns canned stats / error.
type fakeIssueSyncRunner struct {
	called int
	stats  IssueSyncStats
	err    error
}

func (f *fakeIssueSyncRunner) SyncDueConfigs(context.Context) (IssueSyncStats, error) {
	f.called++
	return f.stats, f.err
}

func TestIssueSyncPollJobHandlerReturnsStats(t *testing.T) {
	runner := &fakeIssueSyncRunner{stats: IssueSyncStats{
		Configs: 2, Imported: 5, Updated: 2, Skipped: 1, Failed: 0, ErrConfigs: 0,
	}}
	job := IssueSyncPollJob(runner)

	if job.Name != JobNameIssueSyncPoll {
		t.Errorf("Name = %q, want %q", job.Name, JobNameIssueSyncPoll)
	}
	if job.Handler == nil {
		t.Fatal("Handler is nil")
	}
	// 1-minute cadence so per-config intervals (>= 60s) are the actual gate.
	if job.Cadence != 60000000000 {
		t.Errorf("Cadence = %v, want 1m", job.Cadence)
	}

	res, err := job.Handler(context.Background(), HandlerInput{})
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if runner.called != 1 {
		t.Errorf("runner called %d times, want 1", runner.called)
	}
	if res.RowsAffected != 8 { // imported(5) + updated(2) + skipped(1)
		t.Errorf("RowsAffected = %d, want 8", res.RowsAffected)
	}
	if v, _ := res.Result["imported"].(int); v != 5 {
		t.Errorf("Result.imported = %v, want 5", res.Result["imported"])
	}
	if v, _ := res.Result["updated"].(int); v != 2 {
		t.Errorf("Result.updated = %v, want 2", res.Result["updated"])
	}
}

func TestIssueSyncPollJobHandlerPropagatesRunnerError(t *testing.T) {
	runner := &fakeIssueSyncRunner{err: errors.New("db down")}
	job := IssueSyncPollJob(runner)

	_, err := job.Handler(context.Background(), HandlerInput{})
	if err == nil {
		t.Fatal("expected runner error to propagate, got nil")
	}
}
