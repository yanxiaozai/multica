package scheduler

import (
	"context"
	"time"
)

// JobNameIssueSyncPoll is the canonical job name written to sys_cron_executions
// audit rows. Stable across releases — renaming orphans historic rows.
const JobNameIssueSyncPoll = "issue_sync_poll"

// IssueSyncRunner is the narrow contract this job needs from the issue bridge
// service. Defined here so unit tests can stub it without importing the
// service package (mirrors AutopilotScheduleDispatcher).
type IssueSyncRunner interface {
	SyncDueConfigs(ctx context.Context) (IssueSyncStats, error)
}

// IssueSyncStats mirrors issuebridge.PollStats without the cross-package type
// dependency.
type IssueSyncStats struct {
	Configs    int
	Imported   int
	Skipped    int
	Failed     int
	ErrConfigs int
}

// IssueSyncPollJob returns the JobSpec that drives periodic GitLab issue
// syncing. Every tick, SyncDueConfigs loads every due sync config and runs an
// incremental import. The scheduler's sys_cron_executions lease makes the
// tick single-instance across replicas; per-config idempotency (via
// issue_bridge_item) and the last_poll_at watermark handle the rest.
//
//	cadence:             1m (tick frequently; per-config interval gates actual work)
//	catch_up_mode:       latest_only (a missed tick just runs once next tick)
//	run_timeout:         5m (a slow GitLab shouldn't block the lease forever)
//	stale_timeout:       10m
//	heartbeat_interval:  30s
//	max_attempts:        3
//	retry_backoff:       1m, 5m
//	allow_stale_reentry: true (sync is idempotent, safe to steal a stale lease)
func IssueSyncPollJob(runner IssueSyncRunner) JobSpec {
	return JobSpec{
		Name:              JobNameIssueSyncPoll,
		Cadence:           1 * time.Minute,
		ScheduleDelay:     1 * time.Minute,
		CatchUpMode:       CatchUpLatestOnly,
		CatchUpWindow:     24 * time.Hour,
		RunTimeout:        5 * time.Minute,
		StaleTimeout:      10 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff:      []time.Duration{1 * time.Minute, 5 * time.Minute},
		Scopes:            StaticScopes(ScopeGlobal),
		Handler:           makeIssueSyncPollHandler(runner),
	}
}

func makeIssueSyncPollHandler(runner IssueSyncRunner) Handler {
	return func(ctx context.Context, _ HandlerInput) (HandlerResult, error) {
		stats, err := runner.SyncDueConfigs(ctx)
		if err != nil {
			return HandlerResult{}, err
		}
		return HandlerResult{
			// RowsAffected folds imported + skipped into one "work done"
			// counter for the audit row; per-issue detail lives in Result.
			RowsAffected: int64(stats.Imported + stats.Skipped),
			Result: map[string]any{
				"configs":         stats.Configs,
				"imported":        stats.Imported,
				"skipped":         stats.Skipped,
				"failed":          stats.Failed,
				"error_configs":   stats.ErrConfigs,
			},
		}, nil
	}
}
