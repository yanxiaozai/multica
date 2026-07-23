# GitLab Issue Bridge Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working foundation for a workspace-level GitLab issue bridge: persisted integration config, sync config, default `issue-creator` skill initialization, GitLab connection testing, and typed client/API surfaces.

**Architecture:** Add two workspace-scoped database tables and keep local issue creation untouched in this first slice. Implement a focused backend service for integration config and GitLab connection testing, expose workspace-authenticated HTTP handlers, then add strict-but-lenient frontend schemas and API client methods. Leave polling, remote issue creation, local `done` close, and settings UI as follow-up plans after this foundation is verified.

**Tech Stack:** Go 1.26, Chi handlers, pgx/sqlc, PostgreSQL JSONB, TypeScript, zod, TanStack Query-ready API client.

---

## Scope

This plan implements the foundation only:

- Database schema for `issue_integration` and `issue_sync_config`
- sqlc queries and generated Go code
- Backend integration service with token redaction/encryption boundary
- HTTP handlers for CRUD and GitLab connection test
- Workspace `issue-creator` default skill creation/reuse
- TypeScript types, zod schemas, and API client methods

This plan deliberately does not implement:

- Polling GitLab into Multica
- Creating GitLab issues from new Multica issues
- Closing GitLab issues when Multica issues become `done`
- UI for settings or project sync configuration

## File Map

- Create `server/migrations/127_issue_bridge.up.sql`: add the two foundation tables and indexes.
- Create `server/migrations/127_issue_bridge.down.sql`: drop the tables in dependency order.
- Create `server/pkg/db/queries/issue_integration.sql`: sqlc queries for integrations and sync configs.
- Generate `server/pkg/db/generated/issue_integration.sql.go`: produced by `make sqlc`.
- Create `server/internal/service/issuebridge/default_issue_creator.go`: bundled default `issue-creator` content used when no workspace skill exists.
- Create `server/internal/service/issuebridge/service.go`: business logic for config CRUD, default skill ensure, token handling, and GitLab test call.
- Create `server/internal/service/issuebridge/gitlab.go`: minimal GitLab API client for `/user` and project validation.
- Create `server/internal/service/issuebridge/service_test.go`: service tests with fake HTTP GitLab server.
- Create `server/internal/handler/issue_integration.go`: HTTP request/response models and handlers.
- Create `server/internal/handler/issue_integration_test.go`: route/permission/response tests.
- Modify `server/internal/handler/handler.go`: add `IssueBridgeService` field if handler construction follows the existing service-field pattern.
- Modify `server/cmd/server/router.go`: construct the service and register routes.
- Modify `packages/core/types.ts`: add issue bridge types.
- Modify `packages/core/api/schemas.ts`: add zod schemas and fallbacks.
- Modify `packages/core/api/client.ts`: add methods for integration and sync config APIs.
- Add or modify `packages/core/api/schema.test.ts`: malformed-response coverage for new schemas.

---

### Task 1: Database Schema And sqlc Queries

**Files:**
- Create: `server/migrations/127_issue_bridge.up.sql`
- Create: `server/migrations/127_issue_bridge.down.sql`
- Create: `server/pkg/db/queries/issue_integration.sql`
- Generate: `server/pkg/db/generated/issue_integration.sql.go`

- [ ] **Step 1: Write migration up file**

Create `server/migrations/127_issue_bridge.up.sql`:

```sql
CREATE TABLE issue_integration (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    encrypted_token TEXT NOT NULL DEFAULT '',
    default_issue_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL,
    polling_enabled BOOLEAN NOT NULL DEFAULT false,
    default_poll_interval_seconds INTEGER NOT NULL DEFAULT 300,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issue_integration_provider_check CHECK (provider IN ('gitlab')),
    CONSTRAINT issue_integration_base_url_check CHECK (base_url ~ '^https?://'),
    CONSTRAINT issue_integration_poll_interval_check CHECK (default_poll_interval_seconds >= 60),
    CONSTRAINT issue_integration_config_object_check CHECK (jsonb_typeof(config) = 'object'),
    UNIQUE (workspace_id, provider, name)
);

CREATE INDEX issue_integration_workspace_idx
    ON issue_integration (workspace_id, provider);

CREATE TABLE issue_sync_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES issue_integration(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL,
    scope_id UUID NOT NULL,
    remote_project_ref TEXT NOT NULL,
    sync_enabled BOOLEAN NOT NULL DEFAULT false,
    poll_interval_seconds INTEGER,
    state_mapping JSONB NOT NULL DEFAULT '{"opened":"backlog","closed":"done"}'::jsonb,
    auto_assign_enabled BOOLEAN NOT NULL DEFAULT false,
    default_assignee_type TEXT,
    default_assignee_id UUID,
    last_poll_at TIMESTAMPTZ,
    last_successful_poll_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issue_sync_config_scope_type_check CHECK (scope_type IN ('project', 'repo_resource')),
    CONSTRAINT issue_sync_config_poll_interval_check CHECK (poll_interval_seconds IS NULL OR poll_interval_seconds >= 60),
    CONSTRAINT issue_sync_config_state_mapping_object_check CHECK (jsonb_typeof(state_mapping) = 'object'),
    CONSTRAINT issue_sync_config_assignee_type_check CHECK (
        default_assignee_type IS NULL OR default_assignee_type IN ('agent', 'squad')
    ),
    CONSTRAINT issue_sync_config_auto_assign_pair_check CHECK (
        (auto_assign_enabled = false)
        OR (default_assignee_type IS NOT NULL AND default_assignee_id IS NOT NULL)
    ),
    UNIQUE (workspace_id, scope_type, scope_id)
);

CREATE INDEX issue_sync_config_integration_idx
    ON issue_sync_config (integration_id);

CREATE INDEX issue_sync_config_poll_idx
    ON issue_sync_config (sync_enabled, last_poll_at)
    WHERE sync_enabled = true;
```

- [ ] **Step 2: Write migration down file**

Create `server/migrations/127_issue_bridge.down.sql`:

```sql
DROP TABLE IF EXISTS issue_sync_config;
DROP TABLE IF EXISTS issue_integration;
```

- [ ] **Step 3: Write sqlc queries**

Create `server/pkg/db/queries/issue_integration.sql`:

```sql
-- name: ListIssueIntegrationsByWorkspace :many
SELECT * FROM issue_integration
WHERE workspace_id = $1
ORDER BY provider ASC, name ASC, created_at ASC;

-- name: GetIssueIntegrationInWorkspace :one
SELECT * FROM issue_integration
WHERE id = $1 AND workspace_id = $2;

-- name: GetIssueIntegrationByProviderName :one
SELECT * FROM issue_integration
WHERE workspace_id = $1 AND provider = $2 AND name = $3;

-- name: CreateIssueIntegration :one
INSERT INTO issue_integration (
    workspace_id, provider, name, base_url, encrypted_token,
    default_issue_skill_id, polling_enabled, default_poll_interval_seconds, config
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateIssueIntegration :one
UPDATE issue_integration
SET name = $3,
    base_url = $4,
    encrypted_token = COALESCE(sqlc.narg('encrypted_token'), encrypted_token),
    default_issue_skill_id = $5,
    polling_enabled = $6,
    default_poll_interval_seconds = $7,
    config = $8,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteIssueIntegration :exec
DELETE FROM issue_integration
WHERE id = $1 AND workspace_id = $2;

-- name: ListIssueSyncConfigsByWorkspace :many
SELECT * FROM issue_sync_config
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: ListIssueSyncConfigsByIntegration :many
SELECT * FROM issue_sync_config
WHERE workspace_id = $1 AND integration_id = $2
ORDER BY created_at ASC;

-- name: GetIssueSyncConfigInWorkspace :one
SELECT * FROM issue_sync_config
WHERE id = $1 AND workspace_id = $2;

-- name: CreateIssueSyncConfig :one
INSERT INTO issue_sync_config (
    workspace_id, integration_id, scope_type, scope_id, remote_project_ref,
    sync_enabled, poll_interval_seconds, state_mapping,
    auto_assign_enabled, default_assignee_type, default_assignee_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11
)
RETURNING *;

-- name: UpdateIssueSyncConfig :one
UPDATE issue_sync_config
SET remote_project_ref = $3,
    sync_enabled = $4,
    poll_interval_seconds = $5,
    state_mapping = $6,
    auto_assign_enabled = $7,
    default_assignee_type = $8,
    default_assignee_id = $9,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteIssueSyncConfig :exec
DELETE FROM issue_sync_config
WHERE id = $1 AND workspace_id = $2;
```

- [ ] **Step 4: Generate sqlc code**

Run: `make sqlc`

Expected: new generated file contains `IssueIntegration`, `IssueSyncConfig`, and query methods above.

- [ ] **Step 5: Commit database foundation**

```bash
git add server/migrations/127_issue_bridge.up.sql server/migrations/127_issue_bridge.down.sql server/pkg/db/queries/issue_integration.sql server/pkg/db/generated
git commit -m "feat(issue-bridge): add integration schema"
```

---

### Task 2: Issue Bridge Service And GitLab Test Client

**Files:**
- Create: `server/internal/service/issuebridge/default_issue_creator.go`
- Create: `server/internal/service/issuebridge/gitlab.go`
- Create: `server/internal/service/issuebridge/service.go`
- Create: `server/internal/service/issuebridge/service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `server/internal/service/issuebridge/service_test.go` with tests named:

```go
func TestEnsureDefaultIssueCreatorReusesExistingSkill(t *testing.T) {}
func TestEnsureDefaultIssueCreatorCreatesMissingSkill(t *testing.T) {}
func TestGitLabClientTestConnectionUsesBearerToken(t *testing.T) {}
func TestNormalizeGitLabBaseURLRejectsInvalidURL(t *testing.T) {}
```

The test setup should use existing server DB test helpers if available in `server/internal/handler` tests; otherwise use a small pgxpool fixture matching nearby service tests. The GitLab client test should use `httptest.NewServer` and assert `Authorization: Bearer secret-token`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd server && go test ./internal/service/issuebridge -run Test -count=1`

Expected: FAIL because package/files or functions are not implemented.

- [ ] **Step 3: Add default skill bundle content**

Create `server/internal/service/issuebridge/default_issue_creator.go`:

```go
package issuebridge

const DefaultIssueCreatorName = "issue-creator"

const DefaultIssueCreatorDescription = "GitLab issue creation workflow for structured issue drafts."

const DefaultIssueCreatorContent = `# GitLab Issue 创建工作流

此 workspace skill 用于把 Multica issue 转换成适合提交到 GitLab 的 issue 内容。

## 输出要求

生成 GitLab issue 时，产出：

- 简洁标题
- 问题或需求背景
- 影响范围
- 验收标准或复现场景
- 建议标签

## 默认正文结构

### 背景

说明为什么需要创建这个 issue。

### 影响范围

说明影响的用户、模块、数据或流程。

### 验收标准

使用 GIVEN / WHEN / THEN 或清晰列表描述完成标准。

### 实现建议

如有明确方向，给出建议；没有则保留为空。
`
```

- [ ] **Step 4: Add GitLab client**

Create `server/internal/service/issuebridge/gitlab.go`:

```go
package issuebridge

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"
)

type GitLabClient struct {
    BaseURL    string
    Token      string
    HTTPClient *http.Client
}

type GitLabUser struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
    Name     string `json:"name"`
}

func NormalizeGitLabBaseURL(raw string) (string, error) {
    raw = strings.TrimRight(strings.TrimSpace(raw), "/")
    if raw == "" {
        return "", fmt.Errorf("base_url is required")
    }
    u, err := url.Parse(raw)
    if err != nil || u.Host == "" {
        return "", fmt.Errorf("base_url must be a valid URL")
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return "", fmt.Errorf("base_url must use http or https")
    }
    return raw, nil
}

func (c GitLabClient) TestConnection(ctx context.Context) (GitLabUser, error) {
    base, err := NormalizeGitLabBaseURL(c.BaseURL)
    if err != nil {
        return GitLabUser{}, err
    }
    if strings.TrimSpace(c.Token) == "" {
        return GitLabUser{}, fmt.Errorf("token is required")
    }
    hc := c.HTTPClient
    if hc == nil {
        hc = &http.Client{Timeout: 10 * time.Second}
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v4/user", nil)
    if err != nil {
        return GitLabUser{}, err
    }
    req.Header.Set("Authorization", "Bearer "+c.Token)
    req.Header.Set("Accept", "application/json")
    resp, err := hc.Do(req)
    if err != nil {
        return GitLabUser{}, fmt.Errorf("gitlab request failed: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return GitLabUser{}, fmt.Errorf("gitlab returned status %d", resp.StatusCode)
    }
    var user GitLabUser
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return GitLabUser{}, fmt.Errorf("decode gitlab user: %w", err)
    }
    return user, nil
}
```

- [ ] **Step 5: Add service skeleton**

Create `server/internal/service/issuebridge/service.go` with:

```go
package issuebridge

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type SecretBox interface {
    EncryptString(plaintext string) (string, error)
    DecryptString(ciphertext string) (string, error)
}

type Service struct {
    Queries *db.Queries
    Secrets SecretBox
    Client  func(baseURL, token string) GitLabClient
}

type UpsertIntegrationParams struct {
    WorkspaceID                pgtype.UUID
    Name                       string
    BaseURL                    string
    Token                      string
    DefaultIssueSkillID         pgtype.UUID
    PollingEnabled             bool
    DefaultPollIntervalSeconds int32
    Config                     json.RawMessage
}

func (s *Service) EnsureDefaultIssueCreator(ctx context.Context, workspaceID pgtype.UUID, createdBy pgtype.UUID) (db.Skill, error) {
    existing, err := s.Queries.GetSkillByWorkspaceAndName(ctx, db.GetSkillByWorkspaceAndNameParams{
        WorkspaceID: workspaceID,
        Name:        DefaultIssueCreatorName,
    })
    if err == nil {
        return existing, nil
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        return db.Skill{}, err
    }
    return s.Queries.CreateSkill(ctx, db.CreateSkillParams{
        WorkspaceID: workspaceID,
        Name:        DefaultIssueCreatorName,
        Description: DefaultIssueCreatorDescription,
        Content:     DefaultIssueCreatorContent,
        Config:      []byte(`{"source":"issue_bridge_default"}`),
        CreatedBy:   createdBy,
    })
}

func CleanIntegrationName(name string) string {
    name = strings.TrimSpace(name)
    if name == "" {
        return "GitLab"
    }
    return name
}

func CleanConfig(raw json.RawMessage) []byte {
    if len(raw) == 0 {
        return []byte(`{}`)
    }
    return raw
}

func ValidatePollInterval(seconds int32) error {
    if seconds < 60 {
        return fmt.Errorf("poll interval must be at least 60 seconds")
    }
    return nil
}
```

- [ ] **Step 6: Run service tests**

Run: `cd server && go test ./internal/service/issuebridge -run Test -count=1`

Expected: PASS.

- [ ] **Step 7: Commit service foundation**

```bash
git add server/internal/service/issuebridge
git commit -m "feat(issue-bridge): add gitlab integration service"
```

---

### Task 3: HTTP Handlers And Routes

**Files:**
- Create: `server/internal/handler/issue_integration.go`
- Create: `server/internal/handler/issue_integration_test.go`
- Modify: `server/internal/handler/handler.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Write failing handler tests**

Create `server/internal/handler/issue_integration_test.go` with these test names:

```go
func TestCreateGitLabIssueIntegrationCreatesDefaultSkill(t *testing.T) {}
func TestListIssueIntegrationsDoesNotReturnToken(t *testing.T) {}
func TestCreateIssueSyncConfigRequiresWorkspaceScopedIntegration(t *testing.T) {}
func TestTestIssueIntegrationReturnsGitLabUser(t *testing.T) {}
```

Use existing handler test request helpers such as `newRequest`, `withURLParams`, and `testHandler` if they are available in the package. Assert response JSON excludes raw or encrypted token values.

- [ ] **Step 2: Run handler tests to verify they fail**

Run: `cd server && go test ./internal/handler -run 'Test(CreateGitLabIssueIntegration|ListIssueIntegrations|CreateIssueSyncConfig|TestIssueIntegration)' -count=1`

Expected: FAIL because routes and handlers are missing.

- [ ] **Step 3: Add request and response structs**

Create `server/internal/handler/issue_integration.go` with request/response structs:

```go
type IssueIntegrationResponse struct {
    ID                         string         `json:"id"`
    WorkspaceID                string         `json:"workspace_id"`
    Provider                   string         `json:"provider"`
    Name                       string         `json:"name"`
    BaseURL                    string         `json:"base_url"`
    DefaultIssueSkillID         *string        `json:"default_issue_skill_id"`
    PollingEnabled             bool           `json:"polling_enabled"`
    DefaultPollIntervalSeconds int32          `json:"default_poll_interval_seconds"`
    Config                     map[string]any `json:"config"`
    CreatedAt                  string         `json:"created_at"`
    UpdatedAt                  string         `json:"updated_at"`
}

type CreateGitLabIssueIntegrationRequest struct {
    Name                       string          `json:"name"`
    BaseURL                    string          `json:"base_url"`
    Token                      string          `json:"token"`
    DefaultIssueSkillID         *string         `json:"default_issue_skill_id"`
    PollingEnabled             bool            `json:"polling_enabled"`
    DefaultPollIntervalSeconds int32           `json:"default_poll_interval_seconds"`
    Config                     json.RawMessage `json:"config"`
}

type IssueSyncConfigResponse struct {
    ID                  string         `json:"id"`
    WorkspaceID         string         `json:"workspace_id"`
    IntegrationID       string         `json:"integration_id"`
    ScopeType           string         `json:"scope_type"`
    ScopeID             string         `json:"scope_id"`
    RemoteProjectRef    string         `json:"remote_project_ref"`
    SyncEnabled         bool           `json:"sync_enabled"`
    PollIntervalSeconds *int32         `json:"poll_interval_seconds"`
    StateMapping        map[string]any `json:"state_mapping"`
    AutoAssignEnabled   bool           `json:"auto_assign_enabled"`
    DefaultAssigneeType *string        `json:"default_assignee_type"`
    DefaultAssigneeID   *string        `json:"default_assignee_id"`
    LastPollAt          *string        `json:"last_poll_at"`
    LastSuccessfulPollAt *string       `json:"last_successful_poll_at"`
    LastError           string         `json:"last_error"`
    CreatedAt           string        `json:"created_at"`
    UpdatedAt           string        `json:"updated_at"`
}
```

- [ ] **Step 4: Add route handlers**

Implement handlers:

```go
func (h *Handler) ListIssueIntegrations(w http.ResponseWriter, r *http.Request)
func (h *Handler) CreateGitLabIssueIntegration(w http.ResponseWriter, r *http.Request)
func (h *Handler) UpdateIssueIntegration(w http.ResponseWriter, r *http.Request)
func (h *Handler) DeleteIssueIntegration(w http.ResponseWriter, r *http.Request)
func (h *Handler) TestIssueIntegration(w http.ResponseWriter, r *http.Request)
func (h *Handler) ListIssueSyncConfigs(w http.ResponseWriter, r *http.Request)
func (h *Handler) CreateIssueSyncConfig(w http.ResponseWriter, r *http.Request)
func (h *Handler) UpdateIssueSyncConfig(w http.ResponseWriter, r *http.Request)
func (h *Handler) DeleteIssueSyncConfig(w http.ResponseWriter, r *http.Request)
```

Use existing helpers:

- `requireUserID`
- `workspaceIDFromRequest`
- `parseUUIDOrBadRequest`
- `writeError`
- `writeJSON`
- `uuidToString`
- `timestampToString`

- [ ] **Step 5: Register routes**

Modify `server/cmd/server/router.go` inside the authenticated API route group:

```go
r.Get("/api/issue-integrations", h.ListIssueIntegrations)
r.Post("/api/issue-integrations/gitlab", h.CreateGitLabIssueIntegration)
r.Put("/api/issue-integrations/{id}", h.UpdateIssueIntegration)
r.Delete("/api/issue-integrations/{id}", h.DeleteIssueIntegration)
r.Post("/api/issue-integrations/{id}/test", h.TestIssueIntegration)
r.Get("/api/issue-sync-configs", h.ListIssueSyncConfigs)
r.Post("/api/issue-sync-configs", h.CreateIssueSyncConfig)
r.Put("/api/issue-sync-configs/{id}", h.UpdateIssueSyncConfig)
r.Delete("/api/issue-sync-configs/{id}", h.DeleteIssueSyncConfig)
```

- [ ] **Step 6: Run handler tests**

Run: `cd server && go test ./internal/handler -run 'Test(CreateGitLabIssueIntegration|ListIssueIntegrations|CreateIssueSyncConfig|TestIssueIntegration)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit handler API**

```bash
git add server/internal/handler/issue_integration.go server/internal/handler/issue_integration_test.go server/internal/handler/handler.go server/cmd/server/router.go
git commit -m "feat(issue-bridge): expose integration api"
```

---

### Task 4: TypeScript Types, Schemas, And API Client

**Files:**
- Modify: `packages/core/types.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/schema.test.ts`
- Modify: `packages/core/api/client.ts`

- [ ] **Step 1: Add TypeScript types**

Add to `packages/core/types.ts`:

```ts
export interface IssueIntegration {
  id: string;
  workspace_id: string;
  provider: "gitlab" | string;
  name: string;
  base_url: string;
  default_issue_skill_id: string | null;
  polling_enabled: boolean;
  default_poll_interval_seconds: number;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateGitLabIssueIntegrationRequest {
  name?: string;
  base_url: string;
  token: string;
  default_issue_skill_id?: string | null;
  polling_enabled?: boolean;
  default_poll_interval_seconds?: number;
  config?: Record<string, unknown>;
}

export interface IssueSyncConfig {
  id: string;
  workspace_id: string;
  integration_id: string;
  scope_type: "project" | "repo_resource" | string;
  scope_id: string;
  remote_project_ref: string;
  sync_enabled: boolean;
  poll_interval_seconds: number | null;
  state_mapping: Record<string, string>;
  auto_assign_enabled: boolean;
  default_assignee_type: "agent" | "squad" | null;
  default_assignee_id: string | null;
  last_poll_at: string | null;
  last_successful_poll_at: string | null;
  last_error: string;
  created_at: string;
  updated_at: string;
}

export interface CreateIssueSyncConfigRequest {
  integration_id: string;
  scope_type: "project" | "repo_resource";
  scope_id: string;
  remote_project_ref: string;
  sync_enabled?: boolean;
  poll_interval_seconds?: number | null;
  state_mapping?: Record<string, string>;
  auto_assign_enabled?: boolean;
  default_assignee_type?: "agent" | "squad" | null;
  default_assignee_id?: string | null;
}

export interface TestIssueIntegrationResponse {
  ok: boolean;
  provider: string;
  username: string;
  name: string;
}
```

- [ ] **Step 2: Add schemas and fallbacks**

Add to `packages/core/api/schemas.ts`:

```ts
export const IssueIntegrationSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  provider: z.string(),
  name: z.string(),
  base_url: z.string(),
  default_issue_skill_id: z.string().nullable().optional().default(null),
  polling_enabled: z.boolean().default(false),
  default_poll_interval_seconds: z.number().default(300),
  config: z.record(z.string(), z.unknown()).default({}),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const IssueIntegrationListSchema = z.array(IssueIntegrationSchema).default([]);

export const IssueSyncConfigSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  integration_id: z.string(),
  scope_type: z.string(),
  scope_id: z.string(),
  remote_project_ref: z.string(),
  sync_enabled: z.boolean().default(false),
  poll_interval_seconds: z.number().nullable().optional().default(null),
  state_mapping: z.record(z.string(), z.string()).default({ opened: "backlog", closed: "done" }),
  auto_assign_enabled: z.boolean().default(false),
  default_assignee_type: z.string().nullable().optional().default(null),
  default_assignee_id: z.string().nullable().optional().default(null),
  last_poll_at: z.string().nullable().optional().default(null),
  last_successful_poll_at: z.string().nullable().optional().default(null),
  last_error: z.string().optional().default(""),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const IssueSyncConfigListSchema = z.array(IssueSyncConfigSchema).default([]);

export const TestIssueIntegrationResponseSchema = z.object({
  ok: z.boolean().default(false),
  provider: z.string().default("gitlab"),
  username: z.string().default(""),
  name: z.string().default(""),
}).loose();

export const EMPTY_ISSUE_INTEGRATION = {
  id: "",
  workspace_id: "",
  provider: "gitlab",
  name: "",
  base_url: "",
  default_issue_skill_id: null,
  polling_enabled: false,
  default_poll_interval_seconds: 300,
  config: {},
  created_at: "",
  updated_at: "",
};

export const EMPTY_TEST_ISSUE_INTEGRATION_RESPONSE = {
  ok: false,
  provider: "gitlab",
  username: "",
  name: "",
};
```

- [ ] **Step 3: Add malformed response tests**

In `packages/core/api/schema.test.ts`, add tests:

```ts
it("defaults issue integration optional fields", async () => {
  const parsed = parseWithFallback(
    IssueIntegrationSchema,
    {
      id: "int-1",
      workspace_id: "ws-1",
      provider: "gitlab",
      name: "GitLab",
      base_url: "https://gitlab.example.com",
      created_at: "now",
      updated_at: "now",
    },
    EMPTY_ISSUE_INTEGRATION,
  );
  expect(parsed.polling_enabled).toBe(false);
  expect(parsed.default_poll_interval_seconds).toBe(300);
  expect(parsed.default_issue_skill_id).toBeNull();
});

it("defaults issue sync config optional fields", async () => {
  const parsed = parseWithFallback(
    IssueSyncConfigSchema,
    {
      id: "cfg-1",
      workspace_id: "ws-1",
      integration_id: "int-1",
      scope_type: "project",
      scope_id: "project-1",
      remote_project_ref: "group/project",
      created_at: "now",
      updated_at: "now",
    },
    null,
  );
  expect(parsed?.state_mapping).toEqual({ opened: "backlog", closed: "done" });
  expect(parsed?.poll_interval_seconds).toBeNull();
});
```

- [ ] **Step 4: Add API client methods**

Modify `packages/core/api/client.ts` imports and class methods:

```ts
async listIssueIntegrations(): Promise<IssueIntegration[]> {
  const data = await this.fetch("/api/issue-integrations");
  return parseWithFallback(IssueIntegrationListSchema, data, []);
}

async createGitLabIssueIntegration(
  request: CreateGitLabIssueIntegrationRequest,
): Promise<IssueIntegration> {
  const data = await this.fetch("/api/issue-integrations/gitlab", {
    method: "POST",
    body: JSON.stringify(request),
  });
  return parseWithFallback(IssueIntegrationSchema, data, EMPTY_ISSUE_INTEGRATION);
}

async testIssueIntegration(id: string): Promise<TestIssueIntegrationResponse> {
  const data = await this.fetch(`/api/issue-integrations/${id}/test`, { method: "POST" });
  return parseWithFallback(
    TestIssueIntegrationResponseSchema,
    data,
    EMPTY_TEST_ISSUE_INTEGRATION_RESPONSE,
  );
}

async listIssueSyncConfigs(): Promise<IssueSyncConfig[]> {
  const data = await this.fetch("/api/issue-sync-configs");
  return parseWithFallback(IssueSyncConfigListSchema, data, []);
}
```

- [ ] **Step 5: Run core API tests**

Run: `pnpm --filter @multica/core test -- api/schema.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit frontend API surface**

```bash
git add packages/core/types.ts packages/core/api/schemas.ts packages/core/api/schema.test.ts packages/core/api/client.ts
git commit -m "feat(issue-bridge): add client api types"
```

---

### Task 5: Narrow Verification

**Files:**
- Verify changed backend and core frontend files.

- [ ] **Step 1: Run Go issue bridge service tests**

Run: `cd server && go test ./internal/service/issuebridge -count=1`

Expected: PASS.

- [ ] **Step 2: Run handler tests**

Run: `cd server && go test ./internal/handler -run 'IssueIntegration|IssueSyncConfig' -count=1`

Expected: PASS.

- [ ] **Step 3: Run core tests**

Run: `pnpm --filter @multica/core test -- api/schema.test.ts`

Expected: PASS.

- [ ] **Step 4: Run typecheck for touched TypeScript package**

Run: `pnpm --filter @multica/core typecheck`

Expected: PASS.

- [ ] **Step 5: Commit fixes if verification required any changes**

If verification required changes:

```bash
git add <changed-files>
git commit -m "fix(issue-bridge): stabilize foundation"
```

If no changes were required, do not create an empty commit.

---

## Self-Review

Spec coverage:

- Workspace GitLab integration config: Task 1, Task 2, Task 3, Task 4.
- Workspace token stored server-side and hidden from frontend: Task 1, Task 2, Task 3.
- Default `issue-creator` skill creation/reuse: Task 2, Task 3.
- Project/repo sync config shape: Task 1, Task 3, Task 4.
- State mapping and auto-assignment config fields: Task 1, Task 3, Task 4.
- GitLab API backend path, not `glab`: Task 2 and Task 3.

Deferred by design:

- Polling remote issues into Multica.
- Pushing local issue creation to GitLab.
- Closing GitLab when local issue becomes `done`.
- Settings UI.

No placeholder steps remain in this plan. All deferred items are explicit non-scope for this foundation plan.
