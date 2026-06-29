package issuebridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ImportOpts controls what a single import pass pulls from GitLab.
type ImportOpts struct {
	// AssignedToMe restricts to issues assigned to the token owner
	// (GitLab scope=assigned_to_me). Manual project import sets this;
	// polling keeps it true too so volume stays bounded to "my" issues.
	AssignedToMe bool
	// State filters the GitLab state ("opened"/"closed"/"all"). Imports
	// use "all" so closed issues land via state_mapping; polling uses
	// "all" as well and relies on UpdatedAfter for incrementality.
	State string
	// UpdatedAfter (Phase B) restricts to issues updated since the last
	// successful poll. Zero = no filter (full one-time import).
	UpdatedAfter time.Time
}

// ImportResult tallies one import pass.
type ImportResult struct {
	Imported int // newly created local issues
	Skipped  int // already mapped (idempotent re-run)
	Failed   int // per-issue errors; collected, non-fatal
	Errors   []string
}

// ErrSyncConfigMissing signals the project has no GitLab sync config yet.
// The handler translates this to a 400/409 that tells the user to configure
// the link first.
var ErrSyncConfigMissing = errors.New("project has no gitlab issue sync config; configure the GitLab link first")

// ErrIssueServiceMissing signals the server wiring didn't inject IssueService.
// Operator-facing: the daemon didn't call SetIssueService.
var ErrIssueServiceMissing = errors.New("issue bridge issue service is not configured")

// ImportProjectIssues pulls GitLab issues for the project's sync config and
// creates a local issue for each not-yet-imported one. Idempotent: re-running
// skips GitLab issues already present in issue_bridge_item. The actorUserID
// becomes the creator of the imported issues (the user who triggered import).
func (s *Service) ImportProjectIssues(
	ctx context.Context,
	workspaceID, projectID, actorUserID pgtype.UUID,
	opts ImportOpts,
) (ImportResult, error) {
	if s == nil || s.Queries == nil {
		return ImportResult{}, fmt.Errorf("issue bridge service requires queries")
	}
	if s.IssueService == nil {
		return ImportResult{}, ErrIssueServiceMissing
	}

	cfg, err := s.Queries.GetIssueSyncConfigByScope(ctx, db.GetIssueSyncConfigByScopeParams{
		WorkspaceID: workspaceID,
		ScopeType:   "project",
		ScopeID:     projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportResult{}, ErrSyncConfigMissing
		}
		return ImportResult{}, fmt.Errorf("load sync config: %w", err)
	}

	integration, err := s.Queries.GetIssueIntegrationInWorkspace(ctx, db.GetIssueIntegrationInWorkspaceParams{
		ID:          cfg.IntegrationID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("load integration: %w", err)
	}

	token, err := s.decryptToken(integration.EncryptedToken)
	if err != nil {
		return ImportResult{}, err
	}

	factory := s.ClientFactory
	if factory == nil {
		factory = defaultClientFactory
	}
	client, err := factory(integration.BaseUrl, token)
	if err != nil {
		return ImportResult{}, err
	}

	remoteIssues, err := client.ListProjectIssues(ctx, cfg.RemoteProjectRef, ListIssuesOpts{
		AssignedToMe: opts.AssignedToMe,
		State:        opts.State,
		UpdatedAfter: opts.UpdatedAfter,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("fetch gitlab issues: %w", err)
	}

	stateMap := decodeStateMapping(cfg.StateMapping)
	result := ImportResult{}
	for _, ri := range remoteIssues {
		// Idempotency: skip GitLab issues we already imported.
		if existing, lookupErr := s.Queries.GetIssueBridgeItemByRemote(ctx, db.GetIssueBridgeItemByRemoteParams{
			IntegrationID: cfg.IntegrationID,
			RemoteIid:     ri.IID,
		}); lookupErr == nil && existing.IssueID.Valid {
			result.Skipped++
			continue
		} else if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			result.failed(fmt.Sprintf("iid %d: dedup lookup: %v", ri.IID, lookupErr))
			continue
		}

		params := service.IssueCreateParams{
			WorkspaceID:    workspaceID,
			Title:          ri.Title,
			Description:    pgtype.Text{String: ri.Description, Valid: ri.Description != ""},
			Status:         mapRemoteState(stateMap, ri.State),
			Priority:       "none",
			ProjectID:      projectID,
			CreatorType:    "member",
			CreatorID:      actorUserID,
			AllowDuplicate: true,
		}
		// Auto-assign: stamp the configured agent/squad so the imported
		// issue is actionable (and, for agents, triggers on-assign task
		// enqueue via IssueService.Create).
		if cfg.AutoAssignEnabled && cfg.DefaultAssigneeType.Valid && cfg.DefaultAssigneeID.Valid {
			params.AssigneeType = cfg.DefaultAssigneeType
			params.AssigneeID = cfg.DefaultAssigneeID
		}

		// TODO(issubreidge-ws): pass a BroadcastPayload so the issue:created WS
		// event carries the full issue, not the minimal {issue_id} fallback.
		// With nil BroadcastPayload, IssueService.Create emits {issue_id} only,
		// and the renderer's WS dispatcher (use-realtime-sync.ts issue:created)
		// bails on `if (!issue) return` — so OTHER clients / tabs never receive
		// a live update for imported or polled issues (they see them only after
		// a manual refresh). The importing user's own view is covered because
		// useImportProjectGitLabIssues invalidates the issue caches on settle,
		// but that's per-client and doesn't help teammates or the scheduler's
		// background polls. Proper fix: extract the handler-layer issue→response
		// mapping into a shared helper (or a service-level builder) and supply
		// it here as BroadcastPayload. Tracked as a known gap, not yet done.
		created, createErr := s.IssueService.Create(ctx, params, service.IssueCreateOpts{})
		if createErr != nil {
			// A single bad issue must not abort the whole batch.
			result.failed(fmt.Sprintf("iid %d (%q): %v", ri.IID, ri.Title, createErr))
			continue
		}
		if created.Issue.ID.Valid {
			if _, insErr := s.Queries.CreateIssueBridgeItem(ctx, db.CreateIssueBridgeItemParams{
				WorkspaceID:      workspaceID,
				IssueID:          created.Issue.ID,
				IntegrationID:    cfg.IntegrationID,
				RemoteProjectRef: cfg.RemoteProjectRef,
				RemoteIid:        ri.IID,
				RemoteUrl:        ri.WebURL,
				RemoteUpdatedAt:  pgTimestamp(ri.UpdatedAt),
			}); insErr != nil {
				// The issue was created but we couldn't record the mapping —
				// a re-run will hit the duplicate guard (AllowDuplicate is
				// true, so it creates a second copy). Log loudly so the
				// operator notices the gap; count it as failed.
				slog.Error("issue bridge: created issue but failed to record mapping",
					"issue_id", created.Issue.ID, "remote_iid", ri.IID, "error", insErr)
				result.failed(fmt.Sprintf("iid %d: record mapping: %v", ri.IID, insErr))
				continue
			}
		}
		result.Imported++
	}
	return result, nil
}

func (r *ImportResult) failed(msg string) {
	r.Failed++
	r.Errors = append(r.Errors, msg)
}

// mapRemoteState translates a GitLab issue state ("opened"/"closed") to a
// Multica issue status via the sync config's state_mapping. Unknown states
// fall back to "backlog" so an import never 400s on a new GitLab state.
func mapRemoteState(mapping map[string]string, state string) string {
	if mapped, ok := mapping[state]; ok && mapped != "" {
		return mapped
	}
	return "backlog"
}

// decodeStateMapping parses the sync config's state_mapping JSONB. Returns an
// empty map on any parse failure; mapRemoteState then falls back to backlog.
func decodeStateMapping(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]string{}
	}
	return out
}

// pgTimestamp converts a Go time to the pgtype.Timestamptz sqlc expects,
// preserving zero as NULL.
func pgTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// PollStats summarises one scheduler tick over every due config.
type PollStats struct {
	Configs   int // due configs processed
	Imported  int // total issues created across all configs
	Skipped   int // already-mapped issues skipped
	Failed    int // per-issue failures across all configs
	ErrConfigs int // configs whose whole poll errored (marked failure)
}

// SyncDueConfigs is the scheduler entry point: pick every due sync config,
// run an incremental import for each, and record the poll watermark. A config
// whose import errors is marked failed (last_error set, last_successful_poll_at
// preserved) but does not abort the rest of the tick. Returns aggregate stats
// the scheduler writes to its audit row.
//
// Incrementality: each import passes UpdatedAfter = the config's
// last_successful_poll_at, so only GitLab issues touched since the last good
// poll are re-fetched. Dedup via issue_bridge_item still backstops any overlap.
func (s *Service) SyncDueConfigs(ctx context.Context) (PollStats, error) {
	if s == nil || s.Queries == nil {
		return PollStats{}, fmt.Errorf("issue bridge service requires queries")
	}
	due, err := s.Queries.ListDueIssueSyncConfigs(ctx)
	if err != nil {
		return PollStats{}, fmt.Errorf("list due sync configs: %w", err)
	}

	stats := PollStats{Configs: len(due)}
	for _, cfg := range due {
		// Only project-scoped configs are imported today (repo_resource has
		// no project context to attach issues to). Skip silently rather than
		// marking failure — the config is valid, just not yet supported.
		if cfg.ScopeType != "project" || !cfg.ScopeID.Valid {
			continue
		}
		// last_successful_poll_at drives the incremental window. NULL (never
		// polled) → full import; ImportProjectIssues treats a zero UpdatedAfter
		// as "no filter".
		var updatedAfter time.Time
		if cfg.LastSuccessfulPollAt.Valid {
			updatedAfter = cfg.LastSuccessfulPollAt.Time
		}

		result, importErr := s.ImportProjectIssues(ctx, cfg.WorkspaceID, cfg.ScopeID, systemActorID(), ImportOpts{
			AssignedToMe: true,
			State:        "all",
			UpdatedAfter: updatedAfter,
		})
		if importErr != nil {
			// Whole-config failure (e.g. token revoked, network). Record it
			// and move on — the next tick retries after the interval.
			stats.ErrConfigs++
			msg := truncateErr(importErr.Error(), 500)
			if _, markErr := s.Queries.MarkIssueSyncPollFailure(ctx, db.MarkIssueSyncPollFailureParams{
				ID:        cfg.ID,
				LastError: msg,
			}); markErr != nil {
				slog.Error("issue bridge: mark poll failure", "config_id", cfg.ID, "error", markErr)
			}
			slog.Warn("issue bridge: poll config failed",
				"config_id", cfg.ID, "workspace_id", cfg.WorkspaceID, "error", importErr)
			continue
		}

		stats.Imported += result.Imported
		stats.Skipped += result.Skipped
		stats.Failed += result.Failed
		if _, markErr := s.Queries.MarkIssueSyncPollSuccess(ctx, cfg.ID); markErr != nil {
			slog.Error("issue bridge: mark poll success", "config_id", cfg.ID, "error", markErr)
		}
	}
	return stats, nil
}

// systemActorID is the creator recorded on poll-imported issues. There is no
// human actor for a scheduler tick, so we use the zero UUID — the issue row's
// creator_type stays "member" but creator_id is NULL, which the UI already
// tolerates (system-origin issues render without a creator chip). A dedicated
// "system" creator_type is a follow-up if analytics need to distinguish it.
func systemActorID() pgtype.UUID {
	return pgtype.UUID{}
}

// truncateErr caps a stored error message so a chatty upstream failure can't
// blow up the last_error column or the UI that renders it.
func truncateErr(msg string, max int) string {
	if len(msg) <= max {
		return msg
	}
	return msg[:max]
}
