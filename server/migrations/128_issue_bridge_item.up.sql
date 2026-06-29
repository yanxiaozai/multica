-- issue_bridge_item dedup-maps a Multica issue to the GitLab issue it was
-- imported from. One row per imported GitLab issue; UNIQUE(integration_id,
-- remote_iid) makes re-import / polling idempotent so the same GitLab issue
-- never produces two local issues. The issue table's origin_id is UUID-typed
-- and can't hold a GitLab iid, so this dedicated table is the mapping store.

CREATE TABLE issue_bridge_item (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES issue_integration(id) ON DELETE CASCADE,
    remote_project_ref TEXT NOT NULL,
    remote_iid BIGINT NOT NULL,
    remote_url TEXT NOT NULL DEFAULT '',
    remote_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issue_bridge_item_remote_unique UNIQUE (integration_id, remote_iid)
);

CREATE INDEX idx_issue_bridge_item_issue ON issue_bridge_item(issue_id);
CREATE INDEX idx_issue_bridge_item_workspace ON issue_bridge_item(workspace_id);
