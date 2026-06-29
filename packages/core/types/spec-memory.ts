import type { IssueMetadata } from "./issue";

export interface SpecEpic {
  id: string;
  workspace_id: string;
  key: string;
  title: string;
  description: string;
  stability: string;
  created_at: string;
  updated_at: string;
}

export interface SpecModule {
  id: string;
  workspace_id: string;
  epic_id: string;
  key: string;
  title: string;
  description: string;
  stability: string;
  created_at: string;
  updated_at: string;
}

export interface SpecDocument {
  id: string;
  workspace_id: string;
  epic_id: string | null;
  module_id: string | null;
  doc_kind: string;
  title: string;
  body: string;
  source_path: string;
  created_at: string;
  updated_at: string;
}

export interface SpecIssueMapping {
  id: string;
  workspace_id: string;
  issue_id: string;
  epic_id: string | null;
  module_id: string | null;
  mapping_kind: "primary" | "related" | string;
  reason: string;
  created_at: string;
  updated_at: string;
}

export interface SpecIssueState {
  id: string;
  workspace_id: string;
  issue_id: string;
  status: string;
  owner: string;
  current_stage: string;
  current_loop: string;
  last_result: string;
  open_questions: string[];
  blockers: string[];
  next_handoff: string;
  audit_mode: string;
  audit_skipped: boolean;
  audit_skip_reason: string;
  audit_skipped_by: string;
  audit_skipped_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface SpecDecision {
  id: string;
  workspace_id: string;
  epic_id: string | null;
  module_id: string | null;
  title: string;
  body: string;
  actor: string;
  source_path: string;
  created_at: string;
  updated_at: string;
}

export interface SpecIssueContext {
  issue_id: string;
  state: SpecIssueState | null;
  mappings: SpecIssueMapping[];
  documents: SpecDocument[];
  metadata: IssueMetadata;
}

export interface ListSpecEpicsResponse {
  epics: SpecEpic[];
}

export interface ListSpecModulesResponse {
  modules: SpecModule[];
}

export interface ListSpecDocumentsResponse {
  documents: SpecDocument[];
}

export interface UpdateIssueSpecMappingRequest {
  primary: {
    epic_id?: string | null;
    module_id?: string | null;
    reason?: string;
  };
  related?: Array<{
    epic_id?: string | null;
    module_id?: string | null;
    reason?: string;
  }>;
}

export interface UpdateIssueSpecMappingResponse {
  primary: SpecIssueMapping;
}

export interface UpdateIssueSpecStateRequest {
  status?: string;
  owner?: string;
  current_stage?: string;
  current_loop?: string;
  last_result?: string;
  open_questions?: string[];
  blockers?: string[];
  next_handoff?: string;
  audit_mode?: string;
  audit_skipped?: boolean;
  audit_skip_reason?: string;
  audit_skipped_by?: string;
}

export interface UpdateIssueSpecStateResponse {
  state: SpecIssueState;
}

export interface UpdateSpecDocumentRequest {
  title?: string;
  body: string;
  source_path?: string;
}

export interface UpdateSpecDocumentResponse {
  document: SpecDocument;
}

export interface CreateSpecDecisionRequest {
  epic_id?: string | null;
  module_id?: string | null;
  title: string;
  body?: string;
  actor?: string;
  source_path?: string;
}

export interface CreateSpecDecisionResponse {
  decision: SpecDecision;
}
