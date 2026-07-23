package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/specmem"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Work with local .spec project memory",
}

var specInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .spec memory files",
	RunE:  runSpecInit,
}

var specStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show .spec memory status",
	RunE:  runSpecStatus,
}

var specSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize .spec files with backend spec memory",
	RunE:  runSpecSync,
}

var specReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read .spec memory",
	RunE:  runSpecRead,
}

var specUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update .spec memory state or a document",
	RunE:  runSpecUpdate,
}

var specHandoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Append a handoff note to .spec memory",
	RunE:  runSpecHandoff,
}

var specDecisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Append a decision record to .spec memory",
	RunE:  runSpecDecision,
}

var specStandardCmd = &cobra.Command{
	Use:   "standard",
	Short: "Append a project standard to .spec memory",
	RunE:  runSpecStandard,
}

var specIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Work with .spec issue mappings",
}

var specWorkflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Work with comment-scoped .spec issue workflows",
}

var specIssueBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Create or update an issue-to-spec mapping",
	RunE:  runSpecIssueBind,
}

var specWorkflowListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "List comment-scoped workflows for an issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecWorkflowList,
}

var specWorkflowGetCmd = &cobra.Command{
	Use:   "get <issue-id>",
	Short: "Get a comment-scoped workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecWorkflowGet,
}

var specWorkflowStartCmd = &cobra.Command{
	Use:   "start <issue-id>",
	Short: "Start or replace a comment-scoped workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecWorkflowStart,
}

var specWorkflowUpdateCmd = &cobra.Command{
	Use:   "update <issue-id>",
	Short: "Update a comment-scoped workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecWorkflowUpdate,
}

var specWorkflowResumeCmd = &cobra.Command{
	Use:   "resume <issue-id>",
	Short: "Resume an interrupted comment-scoped workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecWorkflowResume,
}

type specSyncResponse struct {
	Epics             int      `json:"epics"`
	Modules           int      `json:"modules"`
	Documents         int      `json:"documents"`
	Issues            int      `json:"issues"`
	IssueMappings     int      `json:"issue_mappings"`
	Decisions         int      `json:"decisions"`
	SkippedIssueFiles []string `json:"skipped_issue_files"`
}

var newSpecAPIClient = newAPIClient

func init() {
	specCmd.AddCommand(specInitCmd)
	specCmd.AddCommand(specStatusCmd)
	specCmd.AddCommand(specSyncCmd)
	specCmd.AddCommand(specReadCmd)
	specCmd.AddCommand(specUpdateCmd)
	specCmd.AddCommand(specHandoffCmd)
	specCmd.AddCommand(specDecisionCmd)
	specCmd.AddCommand(specStandardCmd)
	specCmd.AddCommand(specIssueCmd)
	specCmd.AddCommand(specWorkflowCmd)
	specIssueCmd.AddCommand(specIssueBindCmd)
	specWorkflowCmd.AddCommand(specWorkflowListCmd)
	specWorkflowCmd.AddCommand(specWorkflowGetCmd)
	specWorkflowCmd.AddCommand(specWorkflowStartCmd)
	specWorkflowCmd.AddCommand(specWorkflowUpdateCmd)
	specWorkflowCmd.AddCommand(specWorkflowResumeCmd)

	for _, c := range []*cobra.Command{specInitCmd, specStatusCmd, specSyncCmd, specReadCmd, specUpdateCmd, specHandoffCmd, specDecisionCmd, specStandardCmd, specIssueBindCmd, specWorkflowListCmd, specWorkflowGetCmd, specWorkflowStartCmd, specWorkflowUpdateCmd, specWorkflowResumeCmd} {
		c.Flags().String("root", "", "Project root (defaults to nearest ancestor with .git, AGENTS.md, CLAUDE.md, or pnpm-workspace.yaml)")
	}
	for _, c := range []*cobra.Command{specStatusCmd, specSyncCmd} {
		c.Flags().String("server-url", "", "Multica server URL (or MULTICA_SERVER_URL)")
		c.Flags().String("workspace-id", "", "Workspace ID (or MULTICA_WORKSPACE_ID)")
		c.Flags().String("profile", "", "CLI config profile")
	}
	for _, c := range []*cobra.Command{specInitCmd, specStatusCmd, specReadCmd, specUpdateCmd, specHandoffCmd, specDecisionCmd, specStandardCmd} {
		c.Flags().String("epic", "", "Spec epic name")
		c.Flags().String("module", "", "Spec module name")
		c.Flags().String("issue", "", "Issue ID for issue execution state")
	}

	specInitCmd.Flags().Bool("force", false, "Overwrite existing template files")
	specInitCmd.Flags().String("output", "table", "Output format: table or json")

	specStatusCmd.Flags().String("output", "table", "Output format: table or json")
	specStatusCmd.Flags().Bool("backend", false, "Read backend spec memory instead of local .spec files")

	specSyncCmd.Flags().Bool("from-files", false, "Import local .spec files into backend spec memory")
	specSyncCmd.Flags().Bool("to-files", false, "Export backend spec memory into local .spec files")
	specSyncCmd.Flags().Bool("force", false, "Overwrite existing local .spec files when exporting from backend")
	specSyncCmd.Flags().String("output", "table", "Output format: table or json")

	specReadCmd.Flags().String("doc", "", "Document to read: issue, glossary, requirements, design, architecture, frontend, backend, testing, review, acceptance, implementation")
	specReadCmd.Flags().String("output", "text", "Output format: text or json")

	specUpdateCmd.Flags().String("doc", "", "Document to replace")
	specUpdateCmd.Flags().String("content", "", "Inline document content")
	specUpdateCmd.Flags().String("content-file", "", "Read document content from a UTF-8 file")
	specUpdateCmd.Flags().String("status", "", "Issue execution status")
	specUpdateCmd.Flags().String("stage", "", "Issue current stage")
	specUpdateCmd.Flags().String("owner", "", "Current issue owner")
	specUpdateCmd.Flags().String("next-handoff", "", "Expected next handoff")
	specUpdateCmd.Flags().StringArray("open-question", nil, "Append an issue-level open question (repeatable)")
	specUpdateCmd.Flags().StringArray("blocker", nil, "Append an issue-level blocker (repeatable)")
	specUpdateCmd.Flags().String("skip-audit-reason", "", "Explicitly skip the default audit gate with a reason")
	specUpdateCmd.Flags().String("actor", "", "Actor name recorded for audit/handoff entries")
	specUpdateCmd.Flags().String("output", "table", "Output format: table or json")

	for _, c := range []*cobra.Command{specHandoffCmd, specDecisionCmd, specStandardCmd} {
		c.Flags().String("content", "", "Inline content")
		c.Flags().String("content-file", "", "Read content from a UTF-8 file")
		c.Flags().String("actor", "", "Actor name recorded with the entry")
		c.Flags().String("output", "table", "Output format: table or json")
	}
	specHandoffCmd.Flags().String("to", "", "Next owner or agent suggestion")
	specHandoffCmd.Flags().String("stage", "", "Issue current stage after handoff")
	specHandoffCmd.Flags().String("status", "", "Issue execution status after handoff")
	specDecisionCmd.Flags().String("title", "", "Decision title")
	specStandardCmd.Flags().String("title", "", "Standard title")

	specIssueBindCmd.Flags().String("issue", "", "Issue ID (required)")
	specIssueBindCmd.Flags().String("title", "", "Issue title")
	specIssueBindCmd.Flags().String("primary", "", "Primary spec mapping as <epic>/<module>")
	specIssueBindCmd.Flags().StringArray("related", nil, "Related spec mapping as <epic>/<module>:<reason> (repeatable)")
	specIssueBindCmd.Flags().String("actor", "", "Actor name recorded with the mapping")
	specIssueBindCmd.Flags().String("output", "table", "Output format: table or json")

	for _, c := range []*cobra.Command{specWorkflowListCmd, specWorkflowGetCmd, specWorkflowStartCmd, specWorkflowUpdateCmd, specWorkflowResumeCmd} {
		c.Flags().String("output", "table", "Output format: table or json")
	}
	for _, c := range []*cobra.Command{specWorkflowGetCmd, specWorkflowStartCmd, specWorkflowUpdateCmd} {
		c.Flags().String("comment", "", "Trigger comment ID for this workflow")
	}
	for _, c := range []*cobra.Command{specWorkflowStartCmd, specWorkflowUpdateCmd} {
		c.Flags().String("intent", "", "Workflow intent: new_request, resume, constraint, question, no_action")
		c.Flags().String("status", "", "Workflow status: active, interrupted, completed, superseded, blocked")
		c.Flags().String("owner", "", "Workflow owner")
		c.Flags().String("stage", "", "Workflow stage")
		c.Flags().String("last-result", "", "Compact last result")
		c.Flags().String("next-action", "", "Compact next action")
		c.Flags().String("parent-workflow", "", "Parent workflow comment ID")
		c.Flags().String("trigger-comment", "", "Trigger comment ID to record")
	}
	specWorkflowResumeCmd.Flags().String("from", "", "Interrupted workflow comment ID to resume")
	specWorkflowResumeCmd.Flags().String("trigger", "", "New trigger comment ID for the resume request")
	specWorkflowResumeCmd.Flags().String("owner", "", "Workflow owner")
	specWorkflowResumeCmd.Flags().String("stage", "", "Workflow stage")
	specWorkflowResumeCmd.Flags().String("next-action", "", "Override inherited next action")
}

func runSpecInit(cmd *cobra.Command, _ []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	force, _ := cmd.Flags().GetBool("force")
	if err := specmem.Init(root, specmem.InitOptions{Epic: epic, Module: module, Issue: issue, Force: force}); err != nil {
		return err
	}
	status, err := specmem.GetStatus(root, specmem.StatusOptions{Epic: epic, Module: module, Issue: issue})
	if err != nil {
		return err
	}
	return printSpecStatus(cmd, status)
}

func runSpecStatus(cmd *cobra.Command, _ []string) error {
	backend, _ := cmd.Flags().GetBool("backend")
	if backend {
		return runSpecBackendStatus(cmd)
	}
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	status, err := specmem.GetStatus(root, specmem.StatusOptions{Epic: epic, Module: module, Issue: issue})
	if err != nil {
		return err
	}
	return printSpecStatus(cmd, status)
}

func runSpecSync(cmd *cobra.Command, _ []string) error {
	fromFiles, _ := cmd.Flags().GetBool("from-files")
	toFiles, _ := cmd.Flags().GetBool("to-files")
	if fromFiles == toFiles {
		return fmt.Errorf("exactly one of --from-files or --to-files is required")
	}
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	if toFiles {
		client, err := newSpecAPIClient(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := cli.APIContext(cmd.Context())
		defer cancel()
		var snapshot specmem.FileSnapshot
		if err := client.GetJSON(ctx, "/api/spec/sync/to-files", &snapshot); err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		summary, err := specmem.WriteFileSnapshot(root, snapshot, force)
		if err != nil {
			return err
		}
		return printSpecFileWriteResult(cmd, summary)
	}
	snapshot, err := specmem.ReadFileSnapshot(root)
	if err != nil {
		return err
	}
	client, err := newSpecAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	var result specSyncResponse
	if err := client.PostJSON(ctx, "/api/spec/sync/from-files", snapshot, &result); err != nil {
		return err
	}
	return printSpecSyncResult(cmd, result)
}

func runSpecBackendStatus(cmd *cobra.Command) error {
	_, _, issue := specScope(cmd)
	client, err := newSpecAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()

	output, _ := cmd.Flags().GetString("output")
	if issue != "" {
		var result map[string]any
		if err := client.GetJSON(ctx, "/api/issues/"+issue+"/spec", &result); err != nil {
			return err
		}
		if output == "json" {
			return cli.PrintJSON(os.Stdout, result)
		}
		rows := [][]string{
			{"backend", "ok"},
			{"issue", issue},
		}
		if state, ok := result["state"].(map[string]any); ok && state != nil {
			rows = append(rows,
				[]string{"status", fmt.Sprint(state["status"])},
				[]string{"stage", fmt.Sprint(state["current_stage"])},
				[]string{"owner", fmt.Sprint(state["owner"])},
			)
		}
		cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
		return nil
	}

	var result map[string]any
	if err := client.GetJSON(ctx, "/api/spec/epics", &result); err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	rows := [][]string{{"backend", "ok"}}
	if epics, ok := result["epics"].([]any); ok {
		rows = append(rows, []string{"epics", fmt.Sprint(len(epics))})
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func runSpecRead(cmd *cobra.Command, _ []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	doc, _ := cmd.Flags().GetString("doc")
	content, err := specmem.Read(root, specmem.ReadOptions{Epic: epic, Module: module, Issue: issue, Doc: doc})
	if err != nil {
		return err
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, map[string]string{"content": content})
	}
	fmt.Fprint(os.Stdout, content)
	return nil
}

func runSpecUpdate(cmd *cobra.Command, _ []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	content, err := resolveSpecContent(cmd)
	if err != nil {
		return err
	}
	doc, _ := cmd.Flags().GetString("doc")
	status, _ := cmd.Flags().GetString("status")
	stage, _ := cmd.Flags().GetString("stage")
	owner, _ := cmd.Flags().GetString("owner")
	nextHandoff, _ := cmd.Flags().GetString("next-handoff")
	openQuestions, _ := cmd.Flags().GetStringArray("open-question")
	blockers, _ := cmd.Flags().GetStringArray("blocker")
	skipAuditReason, _ := cmd.Flags().GetString("skip-audit-reason")
	actor, _ := cmd.Flags().GetString("actor")
	if err := specmem.Update(root, specmem.UpdateOptions{
		Epic:            epic,
		Module:          module,
		Issue:           issue,
		Doc:             doc,
		Content:         content,
		Status:          status,
		Stage:           stage,
		Owner:           owner,
		NextHandoff:     nextHandoff,
		OpenQuestions:   openQuestions,
		Blockers:        blockers,
		SkipAuditReason: skipAuditReason,
		Actor:           actor,
	}); err != nil {
		return err
	}
	return printSpecMutation(cmd, root, epic, module, issue, "updated")
}

func runSpecHandoff(cmd *cobra.Command, _ []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	content, err := resolveSpecContent(cmd)
	if err != nil {
		return err
	}
	actor, _ := cmd.Flags().GetString("actor")
	to, _ := cmd.Flags().GetString("to")
	stage, _ := cmd.Flags().GetString("stage")
	status, _ := cmd.Flags().GetString("status")
	if err := specmem.AppendHandoff(root, specmem.HandoffOptions{Epic: epic, Module: module, Issue: issue, Summary: content, Actor: actor, To: to, Stage: stage, Status: status}); err != nil {
		return err
	}
	return printSpecMutation(cmd, root, epic, module, issue, "handoff appended")
}

func runSpecDecision(cmd *cobra.Command, _ []string) error {
	return runSpecAppend(cmd, "decision appended", specmem.AppendDecision)
}

func runSpecStandard(cmd *cobra.Command, _ []string) error {
	return runSpecAppend(cmd, "standard appended", specmem.AppendStandard)
}

func runSpecAppend(cmd *cobra.Command, action string, fn func(string, specmem.AppendOptions) error) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	epic, module, issue := specScope(cmd)
	content, err := resolveSpecContent(cmd)
	if err != nil {
		return err
	}
	title, _ := cmd.Flags().GetString("title")
	actor, _ := cmd.Flags().GetString("actor")
	if err := fn(root, specmem.AppendOptions{Epic: epic, Module: module, Title: title, Content: content, Actor: actor}); err != nil {
		return err
	}
	return printSpecMutation(cmd, root, epic, module, issue, action)
}

func specRoot(cmd *cobra.Command) (string, error) {
	root, _ := cmd.Flags().GetString("root")
	if root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	resolved, err := specmem.FindRoot(root)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func specScope(cmd *cobra.Command) (string, string, string) {
	epic, _ := cmd.Flags().GetString("epic")
	module, _ := cmd.Flags().GetString("module")
	issue, _ := cmd.Flags().GetString("issue")
	return epic, module, issue
}

func resolveSpecContent(cmd *cobra.Command) (string, error) {
	inline, _ := cmd.Flags().GetString("content")
	filePath, _ := cmd.Flags().GetString("content-file")
	inlineSet := cmd.Flags().Changed("content")
	if inlineSet && filePath != "" {
		return "", fmt.Errorf("--content and --content-file are mutually exclusive")
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read --content-file: %w", err)
		}
		if len(data) == 0 {
			return "", fmt.Errorf("--content-file is empty")
		}
		return string(data), nil
	}
	if inlineSet {
		return inline, nil
	}
	return "", nil
}

func runSpecIssueBind(cmd *cobra.Command, _ []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	issue, _ := cmd.Flags().GetString("issue")
	title, _ := cmd.Flags().GetString("title")
	primary, _ := cmd.Flags().GetString("primary")
	related, _ := cmd.Flags().GetStringArray("related")
	actor, _ := cmd.Flags().GetString("actor")
	if err := specmem.BindIssue(root, specmem.BindIssueOptions{Issue: issue, Title: title, Primary: primary, Related: related, Actor: actor}); err != nil {
		return err
	}
	epic, module := splitSpecPrimary(primary)
	return printSpecMutation(cmd, root, epic, module, issue, "issue bound")
}

func runSpecWorkflowList(cmd *cobra.Command, args []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	workflows, current, err := specmem.ListWorkflows(root, args[0])
	if err != nil {
		return err
	}
	return printSpecWorkflows(cmd, current, workflows)
}

func runSpecWorkflowGet(cmd *cobra.Command, args []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	commentID, _ := cmd.Flags().GetString("comment")
	workflow, err := specmem.GetWorkflow(root, args[0], commentID)
	if err != nil {
		return err
	}
	return printSpecWorkflow(cmd, workflow)
}

func runSpecWorkflowStart(cmd *cobra.Command, args []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	opts, err := specWorkflowOptionsFromFlags(cmd, args[0])
	if err != nil {
		return err
	}
	workflow, err := specmem.StartWorkflow(root, opts)
	if err != nil {
		return err
	}
	return printSpecWorkflow(cmd, workflow)
}

func runSpecWorkflowUpdate(cmd *cobra.Command, args []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	opts, err := specWorkflowOptionsFromFlags(cmd, args[0])
	if err != nil {
		return err
	}
	workflow, err := specmem.UpdateWorkflow(root, opts)
	if err != nil {
		return err
	}
	return printSpecWorkflow(cmd, workflow)
}

func runSpecWorkflowResume(cmd *cobra.Command, args []string) error {
	root, err := specRoot(cmd)
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	trigger, _ := cmd.Flags().GetString("trigger")
	owner, _ := cmd.Flags().GetString("owner")
	stage, _ := cmd.Flags().GetString("stage")
	nextAction, _ := cmd.Flags().GetString("next-action")
	workflow, err := specmem.ResumeWorkflow(root, specmem.ResumeWorkflowOptions{
		Issue:       args[0],
		FromComment: from,
		Trigger:     trigger,
		Owner:       owner,
		Stage:       stage,
		NextAction:  nextAction,
	})
	if err != nil {
		return err
	}
	return printSpecWorkflow(cmd, workflow)
}

func specWorkflowOptionsFromFlags(cmd *cobra.Command, issue string) (specmem.WorkflowOptions, error) {
	commentID, _ := cmd.Flags().GetString("comment")
	intent, _ := cmd.Flags().GetString("intent")
	status, _ := cmd.Flags().GetString("status")
	owner, _ := cmd.Flags().GetString("owner")
	stage, _ := cmd.Flags().GetString("stage")
	lastResult, _ := cmd.Flags().GetString("last-result")
	nextAction, _ := cmd.Flags().GetString("next-action")
	parentWorkflow, _ := cmd.Flags().GetString("parent-workflow")
	triggerComment, _ := cmd.Flags().GetString("trigger-comment")
	return specmem.WorkflowOptions{
		Issue:          issue,
		CommentID:      commentID,
		Intent:         intent,
		Status:         status,
		Owner:          owner,
		Stage:          stage,
		LastResult:     lastResult,
		NextAction:     nextAction,
		ParentWorkflow: parentWorkflow,
		TriggerComment: triggerComment,
	}, nil
}

func splitSpecPrimary(primary string) (string, string) {
	parts := strings.Split(primary, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func printSpecMutation(cmd *cobra.Command, root, epic, module, issue, action string) error {
	status, err := specmem.GetStatus(root, specmem.StatusOptions{Epic: epic, Module: module, Issue: issue})
	if err != nil {
		return err
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, map[string]any{"action": action, "status": status})
	}
	fmt.Fprintf(os.Stdout, "%s: %s\n", strings.Title(action), status.SpecDir)
	return nil
}

func printSpecStatus(cmd *cobra.Command, status specmem.Status) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, status)
	}
	rows := [][]string{
		{"root", status.Root},
		{"spec", boolStatus(status.ProjectIndexExists)},
	}
	if status.Epic != "" {
		rows = append(rows, []string{"epic", status.Epic})
	}
	if status.Module != "" {
		rows = append(rows, []string{"module", status.Module})
		rows = append(rows, []string{"module_index", boolStatus(status.ModuleIndexExists)})
	}
	if status.Issue != "" {
		rows = append(rows, []string{"issue", status.Issue})
		rows = append(rows, []string{"issue_state", boolStatus(status.IssueStateExists)})
		rows = append(rows, []string{"status", status.IssueState.Status})
		rows = append(rows, []string{"stage", status.IssueState.CurrentStage})
		rows = append(rows, []string{"owner", status.IssueState.Owner})
		rows = append(rows, []string{"audit", auditStatus(status.IssueState.Audit)})
	}
	if len(status.Missing) > 0 {
		rows = append(rows, []string{"missing", strings.Join(status.Missing, ", ")})
	}
	if len(status.Warnings) > 0 {
		rows = append(rows, []string{"warnings", strings.Join(status.Warnings, "; ")})
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func printSpecSyncResult(cmd *cobra.Command, result specSyncResponse) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	rows := [][]string{
		{"epics", fmt.Sprint(result.Epics)},
		{"modules", fmt.Sprint(result.Modules)},
		{"documents", fmt.Sprint(result.Documents)},
		{"issues", fmt.Sprint(result.Issues)},
		{"issue_mappings", fmt.Sprint(result.IssueMappings)},
		{"decisions", fmt.Sprint(result.Decisions)},
		{"skipped_issue_files", fmt.Sprint(len(result.SkippedIssueFiles))},
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func printSpecFileWriteResult(cmd *cobra.Command, result specmem.FileWriteSummary) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	rows := [][]string{
		{"epics", fmt.Sprint(result.Epics)},
		{"modules", fmt.Sprint(result.Modules)},
		{"documents", fmt.Sprint(result.Documents)},
		{"issues", fmt.Sprint(result.Issues)},
		{"decisions", fmt.Sprint(result.Decisions)},
		{"skipped_existing", fmt.Sprint(len(result.SkippedExisting))},
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func printSpecWorkflow(cmd *cobra.Command, workflow specmem.CommentWorkflow) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, workflow)
	}
	rows := [][]string{
		{"comment", workflow.CommentID},
		{"intent", workflow.Intent},
		{"status", workflow.Status},
		{"owner", workflow.Owner},
		{"stage", workflow.Stage},
		{"parent", workflow.ParentWorkflow},
		{"next_action", workflow.NextAction},
		{"updated", workflow.UpdatedAt},
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	return nil
}

func printSpecWorkflows(cmd *cobra.Command, current specmem.CurrentWorkflow, workflows []specmem.CommentWorkflow) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, map[string]any{
			"current":   current,
			"workflows": workflows,
		})
	}
	rows := make([][]string, 0, len(workflows)+1)
	rows = append(rows, []string{"current", current.ActiveThread, "", current.Owner, current.Stage, current.ResumeTarget})
	for _, wf := range workflows {
		rows = append(rows, []string{wf.CommentID, wf.Intent, wf.Status, wf.Owner, wf.Stage, wf.NextAction})
	}
	cli.PrintTable(os.Stdout, []string{"COMMENT", "INTENT", "STATUS", "OWNER", "STAGE", "NEXT"}, rows)
	return nil
}

func boolStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "missing"
}

func auditStatus(a specmem.AuditState) string {
	mode := a.Mode
	if mode == "" {
		mode = "required"
	}
	if a.Skipped {
		if a.SkipReason != "" {
			return "skipped: " + a.SkipReason
		}
		return "skipped"
	}
	return mode
}
