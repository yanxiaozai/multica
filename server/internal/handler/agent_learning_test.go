package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createHandlerTestLearningIssue(t *testing.T, agentID string) string {
	t.Helper()

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, creator_type, creator_id, title,
			assignee_type, assignee_id
		)
		VALUES ($1, 'member', $2, $3, 'agent', $4)
		RETURNING id
	`, testWorkspaceID, testUserID, "Learning report issue", agentID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create learning test issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	return issueID
}

func createHandlerTestLearningSkill(t *testing.T, name string, content string) string {
	t.Helper()

	var skillID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO skill (workspace_id, name, description, content, config, created_by)
		VALUES ($1, $2, '', $3, '{}'::jsonb, $4)
		RETURNING id
	`, testWorkspaceID, name, content, testUserID).Scan(&skillID); err != nil {
		t.Fatalf("failed to create learning test skill: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, skillID)
	})
	return skillID
}

func createLearningReportViaHandler(t *testing.T, body map[string]any) AgentLearningReportResponse {
	t.Helper()

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/agent-learning-reports?workspace_id="+testWorkspaceID, body)
	testHandler.CreateAgentLearningReport(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgentLearningReport: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentLearningReportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode learning report response: %v", err)
	}
	return resp
}

func TestCreateAgentLearningReportAndListByIssueAndAgent(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Learning Report Agent", []byte(`{}`))
	issueID := createHandlerTestLearningIssue(t, agentID)
	taskID := createHandlerTestTaskForAgentOnIssue(t, agentID, issueID)
	skillID := createHandlerTestLearningSkill(t, "learning-report-skill", "# Existing skill\n")

	report := createLearningReportViaHandler(t, map[string]any{
		"agent_id": agentID,
		"issue_id": issueID,
		"task_id":  taskID,
		"summary":  "Learned how to keep agent and skill updates scoped.",
		"metadata": map[string]any{"source": "test"},
		"suggestions": []map[string]any{
			{
				"scope":            "personal_agent",
				"risk":             "safe",
				"target_id":        agentID,
				"title":            "Check desktop route",
				"rationale":        "The agent missed a route-specific boundary.",
				"proposed_content": "When changing desktop routes, verify the route exists in both shell and web adapters.",
			},
			{
				"scope":            "workspace_skill",
				"risk":             "review",
				"target_id":        skillID,
				"title":            "Schema guard",
				"rationale":        "The skill should remind agents to update API schemas.",
				"proposed_content": "When adding API endpoints, update shared schemas and client methods in the same change.",
			},
		},
	})

	if report.AgentID != agentID {
		t.Fatalf("expected report agent %q, got %q", agentID, report.AgentID)
	}
	if report.IssueID == nil || *report.IssueID != issueID {
		t.Fatalf("expected report issue %q, got %v", issueID, report.IssueID)
	}
	if report.TaskID == nil || *report.TaskID != taskID {
		t.Fatalf("expected report task %q, got %v", taskID, report.TaskID)
	}
	if len(report.Suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(report.Suggestions))
	}
	if report.Suggestions[0].TargetType != "agent" || report.Suggestions[1].TargetType != "skill" {
		t.Fatalf("expected default target types agent/skill, got %q/%q", report.Suggestions[0].TargetType, report.Suggestions[1].TargetType)
	}

	issueReq := withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/learning-reports?workspace_id="+testWorkspaceID, nil), "id", issueID)
	issueW := httptest.NewRecorder()
	testHandler.ListIssueLearningReports(issueW, issueReq)
	if issueW.Code != http.StatusOK {
		t.Fatalf("ListIssueLearningReports: expected 200, got %d: %s", issueW.Code, issueW.Body.String())
	}
	var issueReports []AgentLearningReportResponse
	if err := json.NewDecoder(issueW.Body).Decode(&issueReports); err != nil {
		t.Fatalf("decode issue learning reports: %v", err)
	}
	if len(issueReports) != 1 || issueReports[0].ID != report.ID {
		t.Fatalf("expected issue list to contain report %q, got %+v", report.ID, issueReports)
	}
	if len(issueReports[0].Suggestions) != 2 {
		t.Fatalf("expected issue list suggestions to be expanded, got %d", len(issueReports[0].Suggestions))
	}

	agentReq := withURLParam(newRequest(http.MethodGet, "/api/agents/"+agentID+"/learning-reports?workspace_id="+testWorkspaceID, nil), "id", agentID)
	agentW := httptest.NewRecorder()
	testHandler.ListAgentLearningReports(agentW, agentReq)
	if agentW.Code != http.StatusOK {
		t.Fatalf("ListAgentLearningReports: expected 200, got %d: %s", agentW.Code, agentW.Body.String())
	}
	var agentReports []AgentLearningReportResponse
	if err := json.NewDecoder(agentW.Body).Decode(&agentReports); err != nil {
		t.Fatalf("decode agent learning reports: %v", err)
	}
	if len(agentReports) != 1 || agentReports[0].ID != report.ID {
		t.Fatalf("expected agent list to contain report %q, got %+v", report.ID, agentReports)
	}
}

func TestGenerateIssueLearningReportFromLatestTask(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Manual Learning Agent", []byte(`{}`))
	issueID := createHandlerTestLearningIssue(t, agentID)
	taskID := createHandlerTestTaskForAgentOnIssue(t, agentID, issueID)

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/learning-report?workspace_id="+testWorkspaceID, nil), "id", issueID)
	testHandler.GenerateIssueLearningReport(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("GenerateIssueLearningReport: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var report AgentLearningReportResponse
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode generated learning report: %v", err)
	}
	if report.AgentID != agentID {
		t.Fatalf("report agent = %q, want %q", report.AgentID, agentID)
	}
	if report.IssueID == nil || *report.IssueID != issueID {
		t.Fatalf("report issue = %v, want %q", report.IssueID, issueID)
	}
	if report.TaskID == nil || *report.TaskID != taskID {
		t.Fatalf("report task = %v, want %q", report.TaskID, taskID)
	}
	if len(report.Suggestions) != 1 {
		t.Fatalf("expected one generated suggestion, got %d", len(report.Suggestions))
	}
	if got := report.Suggestions[0].Scope; got != "personal_agent" {
		t.Fatalf("suggestion scope = %q, want personal_agent", got)
	}
	if got := report.Suggestions[0].Risk; got != "safe" {
		t.Fatalf("suggestion risk = %q, want safe", got)
	}
}

func TestGenerateIssueLearningReportUsesAgentAssigneeWithoutTask(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Manual Learning Assignee", []byte(`{}`))
	issueID := createHandlerTestLearningIssue(t, agentID)

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/learning-report?workspace_id="+testWorkspaceID, nil), "id", issueID)
	testHandler.GenerateIssueLearningReport(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("GenerateIssueLearningReport: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var report AgentLearningReportResponse
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode generated learning report: %v", err)
	}
	if report.AgentID != agentID {
		t.Fatalf("report agent = %q, want %q", report.AgentID, agentID)
	}
	if report.TaskID != nil {
		t.Fatalf("report task = %v, want nil", report.TaskID)
	}
}

func TestGenerateIssueLearningReportUsesChineseForChineseIssue(t *testing.T) {
	agentID := createHandlerTestAgent(t, "中文复盘 Agent", []byte(`{}`))
	issueID := createHandlerTestLearningIssue(t, agentID)
	taskID := createHandlerTestTaskForAgentOnIssue(t, agentID, issueID)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE issue SET title = '员工模块视觉还原' WHERE id = $1
	`, issueID); err != nil {
		t.Fatalf("update issue title: %v", err)
	}

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/learning-report?workspace_id="+testWorkspaceID, nil), "id", issueID)
	testHandler.GenerateIssueLearningReport(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("GenerateIssueLearningReport: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var report AgentLearningReportResponse
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode generated learning report: %v", err)
	}
	if report.TaskID == nil || *report.TaskID != taskID {
		t.Fatalf("report task = %v, want %q", report.TaskID, taskID)
	}
	if !strings.Contains(report.Summary, "手动 Learning Report") || !strings.Contains(report.Summary, "当前 issue 状态") {
		t.Fatalf("expected Chinese report summary, got %q", report.Summary)
	}
	if len(report.Suggestions) != 1 {
		t.Fatalf("expected one generated suggestion, got %d", len(report.Suggestions))
	}
	suggestion := report.Suggestions[0]
	if suggestion.Title != "交接前保留学习经验" {
		t.Fatalf("suggestion title = %q, want Chinese title", suggestion.Title)
	}
	if !strings.Contains(suggestion.Rationale, "最终交接前") {
		t.Fatalf("expected Chinese rationale, got %q", suggestion.Rationale)
	}
	if !strings.Contains(suggestion.ProposedContent, "中文 agent/skill 使用中文") {
		t.Fatalf("expected Chinese proposed content with language guidance, got %q", suggestion.ProposedContent)
	}
}

func TestGenerateIssueLearningReportRejectsIssueWithoutAgent(t *testing.T) {
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, creator_type, creator_id, title)
		VALUES ($1, 'member', $2, 'No agent learning issue')
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create unassigned issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/learning-report?workspace_id="+testWorkspaceID, nil), "id", issueID)
	testHandler.GenerateIssueLearningReport(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GenerateIssueLearningReport: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyAgentEvolutionSuggestionPersonalAgent(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Agent Evolution Target", []byte(`{}`))
	proposed := "Before editing files, check whether the issue names a specific workspace skill."
	report := createLearningReportViaHandler(t, map[string]any{
		"agent_id": agentID,
		"summary":  "Agent should keep skill ownership explicit.",
		"suggestions": []map[string]any{
			{
				"scope":            "personal_agent",
				"risk":             "safe",
				"target_id":        agentID,
				"title":            "Skill ownership check",
				"proposed_content": proposed,
			},
		},
	})

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/agent-evolution-suggestions/"+report.Suggestions[0].ID+"/apply?workspace_id="+testWorkspaceID, nil), "id", report.Suggestions[0].ID)
	testHandler.ApplyAgentEvolutionSuggestion(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyAgentEvolutionSuggestion: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var app AgentEvolutionApplicationResponse
	if err := json.NewDecoder(w.Body).Decode(&app); err != nil {
		t.Fatalf("decode application response: %v", err)
	}
	if app.TargetType != "agent" || app.TargetID != agentID {
		t.Fatalf("expected agent application for %q, got %+v", agentID, app)
	}

	var instructions string
	if err := testPool.QueryRow(context.Background(), `SELECT instructions FROM agent WHERE id = $1`, agentID).Scan(&instructions); err != nil {
		t.Fatalf("load agent instructions: %v", err)
	}
	if !strings.Contains(instructions, "## Agent Evolution") || !strings.Contains(instructions, proposed) || !strings.Contains(instructions, "agent-evolution:suggestion="+report.Suggestions[0].ID) {
		t.Fatalf("agent instructions did not include evolution block: %q", instructions)
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_evolution_suggestion WHERE id = $1`, report.Suggestions[0].ID).Scan(&status); err != nil {
		t.Fatalf("load suggestion status: %v", err)
	}
	if status != "applied" {
		t.Fatalf("expected suggestion status applied, got %q", status)
	}
}

func TestCreateAgentLearningReportAutoAppliesSafePersonalSuggestion(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Auto Evolution Target", []byte(`{}`))
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent SET agent_evolution_enabled = true WHERE id = $1
	`, agentID); err != nil {
		t.Fatalf("enable agent evolution: %v", err)
	}

	proposed := "When a report cites missing tests, add a regression test before editing the implementation."
	report := createLearningReportViaHandler(t, map[string]any{
		"agent_id": agentID,
		"summary":  "Agent should internalize test-first issue fixes.",
		"suggestions": []map[string]any{
			{
				"scope":            "personal_agent",
				"risk":             "safe",
				"target_id":        agentID,
				"title":            "Test-first fix",
				"proposed_content": proposed,
			},
		},
	})

	if len(report.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(report.Suggestions))
	}
	suggestion := report.Suggestions[0]
	if suggestion.Status != "applied" {
		t.Fatalf("expected auto-applied suggestion, got %q", suggestion.Status)
	}
	if len(suggestion.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(suggestion.Applications))
	}

	var instructions string
	if err := testPool.QueryRow(context.Background(), `SELECT instructions FROM agent WHERE id = $1`, agentID).Scan(&instructions); err != nil {
		t.Fatalf("load agent instructions: %v", err)
	}
	if !strings.Contains(instructions, proposed) || !strings.Contains(instructions, "agent-evolution:suggestion="+suggestion.ID) {
		t.Fatalf("agent instructions did not include auto evolution block: %q", instructions)
	}
}

func TestApplyAgentEvolutionSuggestionWorkspaceSkill(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Skill Evolution Reporter", []byte(`{}`))
	skillID := createHandlerTestLearningSkill(t, "skill-evolution-target", "# Skill\n")
	proposed := "When a backend endpoint changes, update generated client types before handing off the issue."
	report := createLearningReportViaHandler(t, map[string]any{
		"agent_id": agentID,
		"summary":  "Skill should encode endpoint/client coupling.",
		"suggestions": []map[string]any{
			{
				"scope":            "workspace_skill",
				"risk":             "review",
				"target_id":        skillID,
				"title":            "Endpoint client coupling",
				"proposed_content": proposed,
			},
		},
	})

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/agent-evolution-suggestions/"+report.Suggestions[0].ID+"/apply?workspace_id="+testWorkspaceID, nil), "id", report.Suggestions[0].ID)
	testHandler.ApplyAgentEvolutionSuggestion(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyAgentEvolutionSuggestion: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var app AgentEvolutionApplicationResponse
	if err := json.NewDecoder(w.Body).Decode(&app); err != nil {
		t.Fatalf("decode application response: %v", err)
	}
	if app.TargetType != "skill" || app.TargetID != skillID {
		t.Fatalf("expected skill application for %q, got %+v", skillID, app)
	}

	var content string
	if err := testPool.QueryRow(context.Background(), `SELECT content FROM skill WHERE id = $1`, skillID).Scan(&content); err != nil {
		t.Fatalf("load skill content: %v", err)
	}
	if !strings.Contains(content, "## Agent Evolution") || !strings.Contains(content, proposed) || !strings.Contains(content, "agent-evolution:suggestion="+report.Suggestions[0].ID) {
		t.Fatalf("skill content did not include evolution block: %q", content)
	}
}

func TestApplyAgentEvolutionSuggestionRejectsManualAndBuiltin(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Rejected Evolution Agent", []byte(`{}`))
	report := createLearningReportViaHandler(t, map[string]any{
		"agent_id": agentID,
		"summary":  "Some suggestions should remain advisory in v1.",
		"suggestions": []map[string]any{
			{
				"scope":            "personal_agent",
				"risk":             "manual",
				"target_id":        agentID,
				"title":            "Manual only",
				"proposed_content": "This needs human rewrite.",
			},
			{
				"scope":            "builtin_skill_candidate",
				"risk":             "review",
				"title":            "Builtin candidate",
				"proposed_content": "This belongs in a bundled skill later.",
			},
		},
	})

	for _, suggestion := range report.Suggestions {
		w := httptest.NewRecorder()
		req := withURLParam(newRequest(http.MethodPost, "/api/agent-evolution-suggestions/"+suggestion.ID+"/apply?workspace_id="+testWorkspaceID, nil), "id", suggestion.ID)
		testHandler.ApplyAgentEvolutionSuggestion(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("ApplyAgentEvolutionSuggestion %s: expected 400, got %d: %s", suggestion.Scope, w.Code, w.Body.String())
		}
	}

	var instructions string
	if err := testPool.QueryRow(context.Background(), `SELECT instructions FROM agent WHERE id = $1`, agentID).Scan(&instructions); err != nil {
		t.Fatalf("load agent instructions: %v", err)
	}
	if strings.Contains(instructions, "## Agent Evolution") {
		t.Fatalf("manual/builtin rejection should not mutate agent instructions: %q", instructions)
	}
}
