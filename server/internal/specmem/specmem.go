package specmem

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DirName          = ".spec"
	defaultAuditMode = "required"
)

var (
	validSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	hashIssueRef = regexp.MustCompile(`^#([0-9]+)$`)
)

var moduleDocFiles = map[string]string{
	"requirements":   "requirements.md",
	"design":         "design.md",
	"architecture":   "architecture.md",
	"frontend":       "frontend.md",
	"backend":        "backend.md",
	"implementation": "implementation.md",
	"testing":        "testing.md",
	"review":         "review.md",
	"acceptance":     "acceptance.md",
}

type InitOptions struct {
	Epic   string
	Module string
	Issue  string
	Title  string
	Force  bool
	Now    time.Time
}

type StatusOptions struct {
	Epic   string
	Module string
	Issue  string
}

type ReadOptions struct {
	Epic   string
	Module string
	Issue  string
	Doc    string
}

type UpdateOptions struct {
	Epic            string
	Module          string
	Issue           string
	Doc             string
	Content         string
	Status          string
	Stage           string
	Owner           string
	NextHandoff     string
	OpenQuestions   []string
	Blockers        []string
	SkipAuditReason string
	Actor           string
	Now             time.Time
}

type HandoffOptions struct {
	Epic    string
	Module  string
	Issue   string
	Summary string
	Actor   string
	To      string
	Stage   string
	Status  string
	Now     time.Time
}

type BindIssueOptions struct {
	Issue   string
	Title   string
	Primary string
	Related []string
	Actor   string
	Now     time.Time
}

type AppendOptions struct {
	Epic    string
	Module  string
	Title   string
	Content string
	Actor   string
	Now     time.Time
}

type WorkflowOptions struct {
	Issue          string
	CommentID      string
	Intent         string
	Status         string
	TriggerComment string
	ParentWorkflow string
	Owner          string
	Stage          string
	LastResult     string
	NextAction     string
	Now            time.Time
}

type ResumeWorkflowOptions struct {
	Issue       string
	FromComment string
	Trigger     string
	Owner       string
	Stage       string
	NextAction  string
	Now         time.Time
}

type AuditState struct {
	Mode       string `json:"mode" yaml:"mode"`
	Skipped    bool   `json:"skipped" yaml:"skipped"`
	SkipReason string `json:"skip_reason,omitempty" yaml:"skip_reason,omitempty"`
	SkippedBy  string `json:"skipped_by,omitempty" yaml:"skipped_by,omitempty"`
	SkippedAt  string `json:"skipped_at,omitempty" yaml:"skipped_at,omitempty"`
}

type CurrentWorkflow struct {
	ActiveThread string `json:"active_thread,omitempty" yaml:"active_thread,omitempty"`
	Owner        string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Stage        string `json:"stage,omitempty" yaml:"stage,omitempty"`
	ResumeTarget string `json:"resume_target,omitempty" yaml:"resume_target,omitempty"`
}

type CommentWorkflow struct {
	CommentID      string `json:"comment_id" yaml:"comment_id"`
	Intent         string `json:"intent" yaml:"intent"`
	Status         string `json:"status" yaml:"status"`
	TriggerComment string `json:"trigger_comment,omitempty" yaml:"trigger_comment,omitempty"`
	ParentWorkflow string `json:"parent_workflow,omitempty" yaml:"parent_workflow,omitempty"`
	Owner          string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Stage          string `json:"stage,omitempty" yaml:"stage,omitempty"`
	LastResult     string `json:"last_result,omitempty" yaml:"last_result,omitempty"`
	NextAction     string `json:"next_action,omitempty" yaml:"next_action,omitempty"`
	UpdatedAt      string `json:"updated_at" yaml:"updated_at"`
}

type IssueState struct {
	Issue            string            `json:"issue" yaml:"issue"`
	Title            string            `json:"title,omitempty" yaml:"title,omitempty"`
	Primary          string            `json:"primary,omitempty" yaml:"primary,omitempty"`
	Related          []string          `json:"related,omitempty" yaml:"related,omitempty"`
	Status           string            `json:"status" yaml:"status"`
	Owner            string            `json:"owner,omitempty" yaml:"owner,omitempty"`
	CurrentStage     string            `json:"current_stage" yaml:"current_stage"`
	CurrentLoop      string            `json:"current_loop,omitempty" yaml:"current_loop,omitempty"`
	LastResult       string            `json:"last_result,omitempty" yaml:"last_result,omitempty"`
	OpenQuestions    []string          `json:"open_questions,omitempty" yaml:"open_questions,omitempty"`
	Blockers         []string          `json:"blockers,omitempty" yaml:"blockers,omitempty"`
	NextHandoff      string            `json:"next_handoff,omitempty" yaml:"next_handoff,omitempty"`
	CurrentWorkflow  CurrentWorkflow   `json:"current_workflow,omitempty" yaml:"current_workflow,omitempty"`
	CommentWorkflows []CommentWorkflow `json:"comment_workflows,omitempty" yaml:"comment_workflows,omitempty"`
	Audit            AuditState        `json:"audit" yaml:"audit"`
	UpdatedAt        string            `json:"updated_at" yaml:"updated_at"`
}

type Status struct {
	Root               string            `json:"root"`
	SpecDir            string            `json:"spec_dir"`
	ProjectIndexExists bool              `json:"project_index_exists"`
	Epic               string            `json:"epic,omitempty"`
	Module             string            `json:"module,omitempty"`
	Issue              string            `json:"issue,omitempty"`
	ModuleIndexExists  bool              `json:"module_index_exists,omitempty"`
	IssueStateExists   bool              `json:"issue_state_exists,omitempty"`
	IssueState         IssueState        `json:"issue_state,omitempty"`
	Docs               map[string]bool   `json:"docs,omitempty"`
	Missing            []string          `json:"missing,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
	Paths              map[string]string `json:"paths,omitempty"`
}

func FindRoot(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if hasAny(abs, ".git", "AGENTS.md", "CLAUDE.md", "pnpm-workspace.yaml") {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("could not find project root from %s", start)
		}
		abs = parent
	}
}

func Init(root string, opts InitOptions) error {
	if err := validateScope(opts.Epic, opts.Module, opts.Issue); err != nil {
		return err
	}
	now := normalizeNow(opts.Now)
	specDir := filepath.Join(root, DirName)
	for _, dir := range []string{
		specDir,
		filepath.Join(specDir, "issues"),
		filepath.Join(specDir, "decisions"),
		filepath.Join(specDir, "standards"),
		filepath.Join(specDir, "epics"),
	} {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	if err := writeFileIfAllowed(filepath.Join(specDir, "index.md"), projectIndexTemplate(now), opts.Force); err != nil {
		return err
	}
	if err := writeFileIfAllowed(filepath.Join(specDir, "glossary.md"), "# Glossary\n\n", opts.Force); err != nil {
		return err
	}
	if opts.Epic != "" {
		epicDir := filepath.Join(specDir, "epics", opts.Epic)
		if err := ensureDir(epicDir); err != nil {
			return err
		}
		if err := writeFileIfAllowed(filepath.Join(epicDir, "00-index.md"), epicIndexTemplate(opts.Epic, now), opts.Force); err != nil {
			return err
		}
	}
	if opts.Module != "" {
		moduleDir := modulePath(root, opts.Epic, opts.Module)
		if err := ensureDir(moduleDir); err != nil {
			return err
		}
		if err := writeFileIfAllowed(filepath.Join(moduleDir, "00-index.md"), moduleIndexTemplate(opts.Epic, opts.Module, now), opts.Force); err != nil {
			return err
		}
	}
	if opts.Issue != "" {
		issue := canonicalIssueRef(opts.Issue)
		state := defaultIssueState(issue, now)
		state.Title = strings.TrimSpace(opts.Title)
		state.Primary = primaryFromScope(opts.Epic, opts.Module)
		if err := writeFileIfAllowed(issuePath(root, issue), issueStateTemplate(state), opts.Force); err != nil {
			return err
		}
	}
	return nil
}

func GetStatus(root string, opts StatusOptions) (Status, error) {
	if err := validateScope(opts.Epic, opts.Module, opts.Issue); err != nil {
		return Status{}, err
	}
	specDir := filepath.Join(root, DirName)
	st := Status{
		Root:    root,
		SpecDir: specDir,
		Paths:   map[string]string{"spec": specDir},
		Docs:    map[string]bool{},
	}
	st.ProjectIndexExists = fileExists(filepath.Join(specDir, "index.md"))
	if !st.ProjectIndexExists {
		st.Missing = append(st.Missing, filepath.Join(DirName, "index.md"))
	}
	if opts.Issue != "" {
		issue := canonicalIssueRef(opts.Issue)
		st.Issue = issue
		path := issuePath(root, issue)
		st.Paths["issue"] = path
		st.IssueStateExists = fileExists(path)
		if !st.IssueStateExists {
			st.Missing = append(st.Missing, filepath.Join(DirName, "issues", issue+".md"))
		} else {
			state, _, err := readIssueState(path, issue)
			if err != nil {
				st.Warnings = append(st.Warnings, err.Error())
			} else {
				st.IssueState = state
				if state.Primary != "" {
					epic, module := splitPrimary(state.Primary)
					if opts.Epic == "" {
						opts.Epic = epic
					}
					if opts.Module == "" {
						opts.Module = module
					}
				}
			}
		}
	}
	if opts.Epic == "" {
		return st, nil
	}
	st.Epic = opts.Epic
	epicIndex := filepath.Join(specDir, "epics", opts.Epic, "00-index.md")
	st.Paths["epic_index"] = epicIndex
	if !fileExists(epicIndex) {
		st.Missing = append(st.Missing, filepath.Join(DirName, "epics", opts.Epic, "00-index.md"))
	}
	if opts.Module == "" {
		return st, nil
	}
	st.Module = opts.Module
	moduleDir := modulePath(root, opts.Epic, opts.Module)
	indexPath := filepath.Join(moduleDir, "00-index.md")
	st.Paths["module_index"] = indexPath
	st.ModuleIndexExists = fileExists(indexPath)
	if !st.ModuleIndexExists {
		st.Missing = append(st.Missing, filepath.Join(DirName, "epics", opts.Epic, opts.Module, "00-index.md"))
	} else if hasIssueExecutionFields(indexPath) {
		st.Warnings = append(st.Warnings, "module 00-index.md contains issue execution fields; move owner/stage/handoff/open questions/blockers to .spec/issues/<issue-id>.md")
	}
	for _, doc := range sortedDocNames() {
		path := filepath.Join(moduleDir, moduleDocFiles[doc])
		st.Docs[doc] = fileExists(path)
		st.Paths[doc] = path
	}
	return st, nil
}

func Read(root string, opts ReadOptions) (string, error) {
	if err := validateScope(opts.Epic, opts.Module, opts.Issue); err != nil {
		return "", err
	}
	if opts.Doc != "" {
		path, err := docPath(root, opts.Epic, opts.Module, opts.Issue, opts.Doc)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		return string(data), nil
	}
	status, err := GetStatus(root, StatusOptions{Epic: opts.Epic, Module: opts.Module, Issue: opts.Issue})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Spec Memory\n\nRoot: `%s`\n\n", status.SpecDir)
	if status.ProjectIndexExists {
		appendFile(&b, filepath.Join(root, DirName, "index.md"), "Project Index")
	}
	if opts.Issue != "" && status.IssueStateExists {
		appendFile(&b, status.Paths["issue"], "Issue State")
	}
	if status.Epic != "" {
		appendFile(&b, filepath.Join(root, DirName, "epics", status.Epic, "00-index.md"), "Epic Index")
	}
	if status.Module != "" {
		appendFile(&b, filepath.Join(modulePath(root, status.Epic, status.Module), "00-index.md"), "Module Index")
		for _, doc := range []string{"requirements", "design", "architecture", "frontend", "backend", "testing", "review", "acceptance"} {
			if status.Docs[doc] {
				appendFile(&b, status.Paths[doc], titleCase(doc))
			}
		}
	}
	return b.String(), nil
}

func Update(root string, opts UpdateOptions) error {
	if err := validateScope(opts.Epic, opts.Module, opts.Issue); err != nil {
		return err
	}
	now := normalizeNow(opts.Now)
	if opts.Doc != "" {
		if opts.Module == "" {
			return fmt.Errorf("--module is required when --doc is set")
		}
		if opts.Content == "" {
			return fmt.Errorf("--content-file or --content is required when --doc is set")
		}
		if err := Init(root, InitOptions{Epic: opts.Epic, Module: opts.Module, Issue: opts.Issue, Now: now}); err != nil {
			return err
		}
		path, err := docPath(root, opts.Epic, opts.Module, opts.Issue, opts.Doc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(opts.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if !opts.hasIssueStateUpdate() {
		return nil
	}
	if opts.Issue == "" {
		return fmt.Errorf("--issue is required when updating issue execution state")
	}
	if err := Init(root, InitOptions{Epic: opts.Epic, Module: opts.Module, Issue: opts.Issue, Now: now}); err != nil {
		return err
	}
	path := issuePath(root, opts.Issue)
	state, body, err := readIssueState(path, opts.Issue)
	if err != nil {
		return err
	}
	if state.Primary == "" {
		state.Primary = primaryFromScope(opts.Epic, opts.Module)
	}
	if opts.Status != "" {
		state.Status = opts.Status
	}
	if opts.Stage != "" {
		state.CurrentStage = opts.Stage
		state.CurrentLoop = opts.Stage
	}
	if opts.Owner != "" {
		state.Owner = opts.Owner
	}
	if opts.NextHandoff != "" {
		state.NextHandoff = opts.NextHandoff
	}
	state.OpenQuestions = appendClean(state.OpenQuestions, opts.OpenQuestions...)
	state.Blockers = appendClean(state.Blockers, opts.Blockers...)
	if opts.SkipAuditReason != "" {
		state.Audit.Mode = defaultAuditMode
		state.Audit.Skipped = true
		state.Audit.SkipReason = opts.SkipAuditReason
		state.Audit.SkippedBy = defaultActor(opts.Actor)
		state.Audit.SkippedAt = now.Format(time.RFC3339)
	}
	state.UpdatedAt = now.Format(time.RFC3339)
	return writeIssueState(path, state, body)
}

func (opts UpdateOptions) hasIssueStateUpdate() bool {
	return opts.Status != "" ||
		opts.Stage != "" ||
		opts.Owner != "" ||
		opts.NextHandoff != "" ||
		len(opts.OpenQuestions) > 0 ||
		len(opts.Blockers) > 0 ||
		opts.SkipAuditReason != ""
}

func AppendHandoff(root string, opts HandoffOptions) error {
	if err := validateScope(opts.Epic, opts.Module, opts.Issue); err != nil {
		return err
	}
	if opts.Issue == "" {
		return fmt.Errorf("--issue is required for handoff")
	}
	content := strings.TrimSpace(opts.Summary)
	if content == "" {
		return fmt.Errorf("content is required")
	}
	now := normalizeNow(opts.Now)
	if err := Init(root, InitOptions{Epic: opts.Epic, Module: opts.Module, Issue: opts.Issue, Now: now}); err != nil {
		return err
	}
	path := issuePath(root, opts.Issue)
	state, body, err := readIssueState(path, opts.Issue)
	if err != nil {
		return err
	}
	if state.Primary == "" {
		state.Primary = primaryFromScope(opts.Epic, opts.Module)
	}
	if opts.To != "" {
		state.NextHandoff = opts.To
	}
	if opts.Stage != "" {
		state.CurrentStage = opts.Stage
		state.CurrentLoop = opts.Stage
	}
	if opts.Status != "" {
		state.Status = opts.Status
	}
	state.UpdatedAt = now.Format(time.RFC3339)
	entry := fmt.Sprintf("\n\n## Handoff %s\n\n- Actor: %s\n", now.Format(time.RFC3339), defaultActor(opts.Actor))
	if opts.To != "" {
		entry += fmt.Sprintf("- Next: %s\n", opts.To)
	}
	entry += "\n" + content + "\n"
	return writeIssueState(path, state, strings.TrimRight(body, "\n")+entry+"\n")
}

func ListWorkflows(root, issue string) ([]CommentWorkflow, CurrentWorkflow, error) {
	if strings.TrimSpace(issue) == "" {
		return nil, CurrentWorkflow{}, fmt.Errorf("issue is required")
	}
	state, _, err := readIssueState(issuePath(root, issue), issue)
	if err != nil {
		return nil, CurrentWorkflow{}, err
	}
	return append([]CommentWorkflow(nil), state.CommentWorkflows...), state.CurrentWorkflow, nil
}

func GetWorkflow(root, issue, commentID string) (CommentWorkflow, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return CommentWorkflow{}, fmt.Errorf("comment is required")
	}
	workflows, _, err := ListWorkflows(root, issue)
	if err != nil {
		return CommentWorkflow{}, err
	}
	for _, wf := range workflows {
		if wf.CommentID == commentID {
			return wf, nil
		}
	}
	return CommentWorkflow{}, fmt.Errorf("workflow %q not found", commentID)
}

func StartWorkflow(root string, opts WorkflowOptions) (CommentWorkflow, error) {
	if err := validateWorkflowOptions(opts, true); err != nil {
		return CommentWorkflow{}, err
	}
	now := normalizeNow(opts.Now)
	if err := Init(root, InitOptions{Issue: opts.Issue, Now: now}); err != nil {
		return CommentWorkflow{}, err
	}
	path := issuePath(root, opts.Issue)
	state, body, err := readIssueState(path, opts.Issue)
	if err != nil {
		return CommentWorkflow{}, err
	}
	entry := CommentWorkflow{
		CommentID:      strings.TrimSpace(opts.CommentID),
		Intent:         defaultWorkflowIntent(opts.Intent),
		Status:         defaultWorkflowStatus(opts.Status),
		TriggerComment: defaultString(opts.TriggerComment, opts.CommentID),
		ParentWorkflow: strings.TrimSpace(opts.ParentWorkflow),
		Owner:          strings.TrimSpace(opts.Owner),
		Stage:          strings.TrimSpace(opts.Stage),
		LastResult:     strings.TrimSpace(opts.LastResult),
		NextAction:     strings.TrimSpace(opts.NextAction),
		UpdatedAt:      now.Format(time.RFC3339),
	}
	state.CommentWorkflows = upsertWorkflow(state.CommentWorkflows, entry)
	state.CurrentWorkflow = CurrentWorkflow{
		ActiveThread: entry.CommentID,
		Owner:        entry.Owner,
		Stage:        entry.Stage,
		ResumeTarget: entry.ParentWorkflow,
	}
	state.UpdatedAt = entry.UpdatedAt
	if err := writeIssueState(path, state, renderWorkflowSections(body, state)); err != nil {
		return CommentWorkflow{}, err
	}
	return entry, nil
}

func UpdateWorkflow(root string, opts WorkflowOptions) (CommentWorkflow, error) {
	if err := validateWorkflowOptions(opts, false); err != nil {
		return CommentWorkflow{}, err
	}
	if strings.TrimSpace(opts.Issue) == "" {
		return CommentWorkflow{}, fmt.Errorf("issue is required")
	}
	commentID := strings.TrimSpace(opts.CommentID)
	if commentID == "" {
		return CommentWorkflow{}, fmt.Errorf("comment is required")
	}
	path := issuePath(root, opts.Issue)
	state, body, err := readIssueState(path, opts.Issue)
	if err != nil {
		return CommentWorkflow{}, err
	}
	idx := workflowIndex(state.CommentWorkflows, commentID)
	if idx < 0 {
		return CommentWorkflow{}, fmt.Errorf("workflow %q not found", commentID)
	}
	now := normalizeNow(opts.Now)
	entry := state.CommentWorkflows[idx]
	applyWorkflowOptions(&entry, opts)
	entry.UpdatedAt = now.Format(time.RFC3339)
	state.CommentWorkflows[idx] = entry
	if state.CurrentWorkflow.ActiveThread == commentID {
		if entry.Owner != "" {
			state.CurrentWorkflow.Owner = entry.Owner
		}
		if entry.Stage != "" {
			state.CurrentWorkflow.Stage = entry.Stage
		}
		if entry.ParentWorkflow != "" {
			state.CurrentWorkflow.ResumeTarget = entry.ParentWorkflow
		}
	}
	state.UpdatedAt = entry.UpdatedAt
	if err := writeIssueState(path, state, renderWorkflowSections(body, state)); err != nil {
		return CommentWorkflow{}, err
	}
	return entry, nil
}

func ResumeWorkflow(root string, opts ResumeWorkflowOptions) (CommentWorkflow, error) {
	if strings.TrimSpace(opts.FromComment) == "" {
		return CommentWorkflow{}, fmt.Errorf("--from is required")
	}
	if strings.TrimSpace(opts.Trigger) == "" {
		return CommentWorkflow{}, fmt.Errorf("--trigger is required")
	}
	from, err := GetWorkflow(root, opts.Issue, opts.FromComment)
	if err != nil {
		return CommentWorkflow{}, err
	}
	return StartWorkflow(root, WorkflowOptions{
		Issue:          opts.Issue,
		CommentID:      opts.Trigger,
		Intent:         "resume",
		Status:         "active",
		TriggerComment: opts.Trigger,
		ParentWorkflow: from.CommentID,
		Owner:          defaultString(opts.Owner, from.Owner),
		Stage:          defaultString(opts.Stage, from.Stage),
		LastResult:     from.LastResult,
		NextAction:     defaultString(opts.NextAction, from.NextAction),
		Now:            opts.Now,
	})
}

func BindIssue(root string, opts BindIssueOptions) error {
	if strings.TrimSpace(opts.Issue) == "" {
		return fmt.Errorf("--issue is required")
	}
	if err := validateScope("", "", opts.Issue); err != nil {
		return err
	}
	primary := strings.TrimSpace(opts.Primary)
	if primary != "" {
		epic, module := splitPrimary(primary)
		if epic == "" || module == "" {
			return fmt.Errorf("--primary must use <epic>/<module>")
		}
		if err := validateScope(epic, module, ""); err != nil {
			return err
		}
	}
	now := normalizeNow(opts.Now)
	if err := Init(root, InitOptions{Issue: opts.Issue, Title: opts.Title, Now: now}); err != nil {
		return err
	}
	path := issuePath(root, opts.Issue)
	state, body, err := readIssueState(path, opts.Issue)
	if err != nil {
		return err
	}
	if opts.Title != "" {
		state.Title = strings.TrimSpace(opts.Title)
	}
	if primary != "" {
		state.Primary = primary
	}
	state.Related = appendClean(nil, opts.Related...)
	state.UpdatedAt = now.Format(time.RFC3339)
	return writeIssueState(path, state, body)
}

func AppendDecision(root string, opts AppendOptions) error {
	return appendToProjectDoc(root, opts, "decisions")
}

func AppendStandard(root string, opts AppendOptions) error {
	return appendToProjectDoc(root, opts, "standards")
}

func validateScope(epic, module, issue string) error {
	if epic == "" && module != "" {
		return fmt.Errorf("--epic is required when --module is set")
	}
	if err := validateSegment("epic", epic); err != nil {
		return err
	}
	if err := validateSegment("module", module); err != nil {
		return err
	}
	if err := validateSegment("issue", canonicalIssueRef(issue)); err != nil {
		return err
	}
	return nil
}

func validateWorkflowOptions(opts WorkflowOptions, requireIntent bool) error {
	if canonicalIssueRef(opts.Issue) == "" {
		return fmt.Errorf("issue is required")
	}
	if strings.TrimSpace(opts.CommentID) == "" {
		return fmt.Errorf("comment is required")
	}
	if requireIntent && strings.TrimSpace(opts.Intent) == "" {
		return fmt.Errorf("intent is required")
	}
	if opts.Intent != "" && !validWorkflowIntent(opts.Intent) {
		return fmt.Errorf("invalid intent %q", opts.Intent)
	}
	if opts.Status != "" && !validWorkflowStatus(opts.Status) {
		return fmt.Errorf("invalid status %q", opts.Status)
	}
	return nil
}

func validWorkflowIntent(intent string) bool {
	switch strings.TrimSpace(intent) {
	case "new_request", "resume", "constraint", "question", "no_action":
		return true
	default:
		return false
	}
}

func validWorkflowStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "active", "interrupted", "completed", "superseded", "blocked":
		return true
	default:
		return false
	}
}

func defaultWorkflowIntent(intent string) string {
	intent = strings.TrimSpace(intent)
	if intent == "" {
		return "new_request"
	}
	return intent
}

func defaultWorkflowStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "active"
	}
	return status
}

func workflowIndex(workflows []CommentWorkflow, commentID string) int {
	for i, wf := range workflows {
		if wf.CommentID == commentID {
			return i
		}
	}
	return -1
}

func upsertWorkflow(workflows []CommentWorkflow, entry CommentWorkflow) []CommentWorkflow {
	if idx := workflowIndex(workflows, entry.CommentID); idx >= 0 {
		workflows[idx] = entry
		return workflows
	}
	return append(workflows, entry)
}

func applyWorkflowOptions(entry *CommentWorkflow, opts WorkflowOptions) {
	if opts.Intent != "" {
		entry.Intent = opts.Intent
	}
	if opts.Status != "" {
		entry.Status = opts.Status
	}
	if opts.TriggerComment != "" {
		entry.TriggerComment = strings.TrimSpace(opts.TriggerComment)
	}
	if opts.ParentWorkflow != "" {
		entry.ParentWorkflow = strings.TrimSpace(opts.ParentWorkflow)
	}
	if opts.Owner != "" {
		entry.Owner = strings.TrimSpace(opts.Owner)
	}
	if opts.Stage != "" {
		entry.Stage = strings.TrimSpace(opts.Stage)
	}
	if opts.LastResult != "" {
		entry.LastResult = strings.TrimSpace(opts.LastResult)
	}
	if opts.NextAction != "" {
		entry.NextAction = strings.TrimSpace(opts.NextAction)
	}
}

func validateSegment(label, value string) error {
	if value == "" {
		return nil
	}
	if strings.Contains(value, "..") || strings.ContainsAny(value, `/\`) || filepath.IsAbs(value) || !validSegment.MatchString(value) {
		return fmt.Errorf("invalid %s %q; use an ASCII name like feature-auth or 2026-q3", label, value)
	}
	return nil
}

func docPath(root, epic, module, issue, doc string) (string, error) {
	switch doc {
	case "issue":
		if issue == "" {
			return "", fmt.Errorf("--issue is required when --doc issue is set")
		}
		return issuePath(root, issue), nil
	case "glossary":
		return filepath.Join(root, DirName, "glossary.md"), nil
	}
	if module == "" {
		return "", fmt.Errorf("--module is required when --doc is set")
	}
	file, ok := moduleDocFiles[doc]
	if !ok {
		return "", fmt.Errorf("unknown doc %q; valid docs: %s", doc, strings.Join(sortedDocNames(), ", "))
	}
	return filepath.Join(modulePath(root, epic, module), file), nil
}

func modulePath(root, epic, module string) string {
	return filepath.Join(root, DirName, "epics", epic, module)
}

func issuePath(root, issue string) string {
	return filepath.Join(root, DirName, "issues", canonicalIssueRef(issue)+".md")
}

func defaultIssueState(issue string, now time.Time) IssueState {
	issue = canonicalIssueRef(issue)
	return IssueState{
		Issue:        issue,
		Status:       "todo",
		CurrentStage: "requirements",
		CurrentLoop:  "requirements",
		LastResult:   "pending",
		Audit: AuditState{
			Mode: defaultAuditMode,
		},
		UpdatedAt: now.Format(time.RFC3339),
	}
}

func readIssueState(path, issue string) (IssueState, string, error) {
	issue = canonicalIssueRef(issue)
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultIssueState(issue, time.Now()), "", fmt.Errorf("read issue state: %w", err)
	}
	state := defaultIssueState(issue, time.Now())
	body := string(data)
	if bytes.HasPrefix(data, []byte("---\n")) {
		rest := data[len("---\n"):]
		if end := bytes.Index(rest, []byte("\n---")); end >= 0 {
			fm := rest[:end]
			bodyStart := end + len("\n---")
			if bodyStart < len(rest) && rest[bodyStart] == '\n' {
				bodyStart++
			}
			if err := yaml.Unmarshal(fm, &state); err != nil {
				return state, string(rest[bodyStart:]), fmt.Errorf("parse issue frontmatter: %w", err)
			}
			body = string(rest[bodyStart:])
		}
	}
	state.Issue = issue
	if state.Audit.Mode == "" {
		state.Audit.Mode = defaultAuditMode
	}
	return state, body, nil
}

func canonicalIssueRef(issue string) string {
	issue = strings.TrimSpace(issue)
	if matches := hashIssueRef.FindStringSubmatch(issue); len(matches) == 2 {
		return matches[1]
	}
	return issue
}

func writeIssueState(path string, state IssueState, body string) error {
	buf, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode issue frontmatter: %w", err)
	}
	content := "---\n" + string(buf) + "---\n" + strings.TrimLeft(body, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write issue state: %w", err)
	}
	return nil
}

func appendToProjectDoc(root string, opts AppendOptions, dir string) error {
	content := strings.TrimSpace(opts.Content)
	if content == "" {
		return fmt.Errorf("content is required")
	}
	now := normalizeNow(opts.Now)
	if err := Init(root, InitOptions{Epic: opts.Epic, Module: opts.Module, Now: now}); err != nil {
		return err
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = titleCase(strings.TrimSuffix(dir, "s"))
	}
	name := slug(title)
	if name == "" {
		name = dir
	}
	path := filepath.Join(root, DirName, dir, now.Format("2006-01-02")+"-"+name+".md")
	body := fmt.Sprintf("# %s\n\n- Date: %s\n- Actor: %s\n", title, now.Format(time.RFC3339), defaultActor(opts.Actor))
	if opts.Epic != "" {
		body += fmt.Sprintf("- Epic: %s\n", opts.Epic)
	}
	if opts.Module != "" {
		body += fmt.Sprintf("- Module: %s\n", opts.Module)
	}
	body += "\n" + content + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func appendFile(b *strings.Builder, path, title string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	b.Write(data)
	if !strings.HasSuffix(string(data), "\n") {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func renderWorkflowSections(body string, state IssueState) string {
	body = stripMarkdownSection(body, "Current Workflow")
	body = stripMarkdownSection(body, "Comment Workflows")
	body = strings.TrimRight(body, "\n")
	if body != "" {
		body += "\n\n"
	}

	var b strings.Builder
	b.WriteString(body)
	b.WriteString("## Current Workflow\n\n")
	fmt.Fprintf(&b, "Active thread: %s\n", state.CurrentWorkflow.ActiveThread)
	fmt.Fprintf(&b, "Owner: %s\n", state.CurrentWorkflow.Owner)
	fmt.Fprintf(&b, "Stage: %s\n", state.CurrentWorkflow.Stage)
	fmt.Fprintf(&b, "Resume target: %s\n\n", state.CurrentWorkflow.ResumeTarget)
	b.WriteString("## Comment Workflows\n\n")
	if len(state.CommentWorkflows) == 0 {
		b.WriteString("### <trigger_comment_id>\n\n")
		b.WriteString("Intent: new_request | resume | constraint | question | no_action\n")
		b.WriteString("Status: active | interrupted | completed | superseded | blocked\n")
		b.WriteString("Trigger comment: <trigger_comment_id>\n")
		b.WriteString("Parent workflow:\n")
		b.WriteString("Owner:\n")
		b.WriteString("Stage:\n")
		b.WriteString("Last result:\n")
		b.WriteString("Next action:\n")
		b.WriteString("Updated at:\n")
		return b.String()
	}
	for _, wf := range state.CommentWorkflows {
		fmt.Fprintf(&b, "### %s\n\n", wf.CommentID)
		fmt.Fprintf(&b, "Intent: %s\n", wf.Intent)
		fmt.Fprintf(&b, "Status: %s\n", wf.Status)
		fmt.Fprintf(&b, "Trigger comment: %s\n", wf.TriggerComment)
		fmt.Fprintf(&b, "Parent workflow: %s\n", wf.ParentWorkflow)
		fmt.Fprintf(&b, "Owner: %s\n", wf.Owner)
		fmt.Fprintf(&b, "Stage: %s\n", wf.Stage)
		fmt.Fprintf(&b, "Last result: %s\n", wf.LastResult)
		fmt.Fprintf(&b, "Next action: %s\n", wf.NextAction)
		fmt.Fprintf(&b, "Updated at: %s\n\n", wf.UpdatedAt)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func stripMarkdownSection(body, heading string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## "+heading {
			start = i
			break
		}
	}
	if start < 0 {
		return body
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, lines[end:]...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return nil
}

func writeFileIfAllowed(path, content string, force bool) error {
	if !force && fileExists(path) {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasAny(dir string, names ...string) bool {
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func sortedDocNames() []string {
	names := make([]string, 0, len(moduleDocFiles))
	for name := range moduleDocFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func appendClean(existing []string, values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+len(values))
	for _, v := range append(existing, values...) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func normalizeNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func defaultActor(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "agent"
	}
	return actor
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func primaryFromScope(epic, module string) string {
	if epic == "" || module == "" {
		return ""
	}
	return epic + "/" + module
}

func splitPrimary(primary string) (string, string) {
	parts := strings.Split(primary, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func hasIssueExecutionFields(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	for _, marker := range []string{"current_stage:", "next_handoff:", "open_questions:", "blockers:", "## Next Handoff", "## LOOP Chain"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func projectIndexTemplate(now time.Time) string {
	return fmt.Sprintf(`# Spec Memory

This directory is the durable project memory for agent work.

- Created: %s
- Issue state: issues/<issue-id>.md
- Durable modules: epics/<epic>/<module>

`, now.Format(time.RFC3339))
}

func epicIndexTemplate(epic string, now time.Time) string {
	return fmt.Sprintf(`# %s

Epic-level durable context and module index.

- Created: %s

`, epic, now.Format(time.RFC3339))
}

func moduleIndexTemplate(epic, module string, now time.Time) string {
	return fmt.Sprintf(`# %s

## Document Info

- Epic: %s
- Module: %s
- Stability: draft
- Created: %s

## Scope

Describe the durable module scope here.

## Documents

- requirements.md
- design.md
- architecture.md
- frontend.md
- backend.md
- testing.md
- review.md
- acceptance.md

## Related Issues

Add links to .spec/issues/<issue-id>.md files when issues affect this module.

`, module, epic, module, now.Format(time.RFC3339))
}

func issueStateTemplate(state IssueState) string {
	buf, _ := yaml.Marshal(state)
	return fmt.Sprintf(`---
%s---
# Issue %s%s

## Mapping

Primary: %s
Related:

## Execution State

Status: %s
Owner: %s
Current stage: %s
Updated at: %s

## LOOP Chain

Current loop: %s
Last result: %s
Open fixes:

## Open Questions

## Blockers

## Current Workflow

Active thread:
Owner: %s
Stage: %s
Resume target:

## Comment Workflows

### <trigger_comment_id>

Intent: new_request | resume | constraint | question | no_action
Status: active | interrupted | completed | superseded | blocked
Trigger comment: <trigger_comment_id>
Parent workflow:
Owner: %s
Stage: %s
Last result:
Next action:
Updated at: %s

## Next Handoff

## Audit

Mode: %s
Skipped: %t
Reason: %s
`, string(buf), state.Issue, issueTitleSuffix(state.Title), state.Primary, state.Status, state.Owner, state.CurrentStage, state.UpdatedAt, state.CurrentLoop, state.LastResult, state.Owner, state.CurrentStage, state.Owner, state.CurrentStage, state.UpdatedAt, state.Audit.Mode, state.Audit.Skipped, state.Audit.SkipReason)
}

func issueTitleSuffix(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	return ": " + title
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
