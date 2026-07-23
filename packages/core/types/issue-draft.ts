export type IssueDraftStatus =
  | "clarifying"
  | "delegating"
  | "drafting"
  | "ready_for_review"
  | "creating"
  | "created"
  | "failed"
  | "cancelled"
  | string;

export interface IssueDraftSession {
  id: string;
  workspace_id: string;
  project_id: string;
  squad_id: string;
  leader_agent_id: string;
  primary_project_resource_id: string;
  primary_local_path_snapshot: string;
  status: IssueDraftStatus;
  created_by: string;
  created_issue_id: string | null;
  remote_issue_url: string;
  spec_file_path: string;
  git_commit_sha: string;
  last_error: string;
  created_at: string;
  updated_at: string;
}

export interface IssueDraftMessage {
  id: string;
  session_id: string;
  workspace_id: string;
  author_type: string;
  author_id: string | null;
  message_type: string;
  content: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface IssueDraftMemberTask {
  id: string;
  session_id: string;
  workspace_id: string;
  agent_id: string;
  task_id: string | null;
  status: string;
  skill_basis: string;
  read_scope: Record<string, unknown>;
  findings: string;
  error: string;
  created_at: string;
  updated_at: string;
}

export interface IssueDraftArtifact {
  id: string;
  session_id: string;
  workspace_id: string;
  artifact_type: "detailed_spec" | "multica_issue" | "remote_issue" | string;
  revision: number;
  content: string;
  generated_by_agent_id: string | null;
  created_at: string;
}

export interface IssueDraftConfirmStep {
  session_id: string;
  workspace_id: string;
  step: string;
  status: string;
  external_id: string;
  result_metadata: Record<string, unknown>;
  error: string;
  created_at: string;
  updated_at: string;
}

export interface IssueDraftBundle {
  session: IssueDraftSession;
  messages: IssueDraftMessage[];
  member_tasks: IssueDraftMemberTask[];
  artifacts: IssueDraftArtifact[];
  confirm_steps: IssueDraftConfirmStep[];
}

export interface ListIssueDraftSessionsResponse {
  sessions: IssueDraftSession[];
}

export interface CreateIssueDraftRequest {
  project_id: string;
  squad_id: string;
  initial_message?: string;
}

export interface AppendIssueDraftMessageRequest {
  content: string;
  metadata?: Record<string, unknown>;
}
