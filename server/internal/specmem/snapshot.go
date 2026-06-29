package specmem

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileSnapshot struct {
	Epics             []FileEpic     `json:"epics"`
	Issues            []FileIssue    `json:"issues"`
	Decisions         []FileDecision `json:"decisions"`
	SkippedIssueFiles []string       `json:"skipped_issue_files"`
}

type FileEpic struct {
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Stability   string         `json:"stability"`
	Documents   []FileDocument `json:"documents"`
	Modules     []FileModule   `json:"modules"`
}

type FileModule struct {
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Stability   string         `json:"stability"`
	Documents   []FileDocument `json:"documents"`
}

type FileDocument struct {
	DocKind    string `json:"doc_kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	SourcePath string `json:"source_path"`
}

type FileDecision struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	Actor      string `json:"actor"`
	SourcePath string `json:"source_path"`
}

type FileIssue struct {
	Issue         string     `json:"issue"`
	Title         string     `json:"title"`
	Primary       string     `json:"primary"`
	Related       []string   `json:"related"`
	Status        string     `json:"status"`
	Owner         string     `json:"owner"`
	CurrentStage  string     `json:"current_stage"`
	CurrentLoop   string     `json:"current_loop"`
	LastResult    string     `json:"last_result"`
	OpenQuestions []string   `json:"open_questions"`
	Blockers      []string   `json:"blockers"`
	NextHandoff   string     `json:"next_handoff"`
	Audit         AuditState `json:"audit"`
	UpdatedAt     string     `json:"updated_at"`
	SourcePath    string     `json:"source_path"`
}

type FileWriteSummary struct {
	Epics           int      `json:"epics"`
	Modules         int      `json:"modules"`
	Documents       int      `json:"documents"`
	Issues          int      `json:"issues"`
	Decisions       int      `json:"decisions"`
	SkippedExisting []string `json:"skipped_existing"`
}

func ReadFileSnapshot(root string) (FileSnapshot, error) {
	specDir := filepath.Join(root, DirName)
	var snap FileSnapshot

	epicsDir := filepath.Join(specDir, "epics")
	epicEntries, err := os.ReadDir(epicsDir)
	if err != nil && !os.IsNotExist(err) {
		return FileSnapshot{}, fmt.Errorf("read epics: %w", err)
	}
	for _, entry := range epicEntries {
		if !entry.IsDir() {
			continue
		}
		epic, err := readFileEpic(root, entry.Name())
		if err != nil {
			return FileSnapshot{}, err
		}
		snap.Epics = append(snap.Epics, epic)
	}
	sort.Slice(snap.Epics, func(i, j int) bool { return snap.Epics[i].Key < snap.Epics[j].Key })

	decisions, err := readFileDecisions(root)
	if err != nil {
		return FileSnapshot{}, err
	}
	snap.Decisions = decisions

	issues, skippedIssueFiles, err := readFileIssues(root)
	if err != nil {
		return FileSnapshot{}, err
	}
	snap.Issues = issues
	snap.SkippedIssueFiles = skippedIssueFiles
	return snap, nil
}

func WriteFileSnapshot(root string, snap FileSnapshot, force bool) (FileWriteSummary, error) {
	var summary FileWriteSummary
	if err := ensureDir(filepath.Join(root, DirName)); err != nil {
		return summary, err
	}
	for _, epic := range snap.Epics {
		epicKey := safeSnapshotSegment(epic.Key, "epic")
		if err := ensureDir(filepath.Join(root, DirName, "epics", epicKey)); err != nil {
			return summary, err
		}
		summary.Epics++
		for _, doc := range epic.Documents {
			path := epicDocumentPath(root, epicKey, doc)
			wrote, err := writeSnapshotFile(path, doc.Body, force)
			if err != nil {
				return summary, err
			}
			if wrote {
				summary.Documents++
			} else {
				summary.SkippedExisting = append(summary.SkippedExisting, relPath(root, path))
			}
		}
		for _, module := range epic.Modules {
			moduleKey := safeSnapshotSegment(module.Key, "module")
			if err := ensureDir(filepath.Join(root, DirName, "epics", epicKey, moduleKey)); err != nil {
				return summary, err
			}
			summary.Modules++
			for _, doc := range module.Documents {
				path := moduleDocumentPath(root, epicKey, moduleKey, doc)
				wrote, err := writeSnapshotFile(path, doc.Body, force)
				if err != nil {
					return summary, err
				}
				if wrote {
					summary.Documents++
				} else {
					summary.SkippedExisting = append(summary.SkippedExisting, relPath(root, path))
				}
			}
		}
	}
	for _, decision := range snap.Decisions {
		path := decisionPath(root, decision)
		wrote, err := writeSnapshotFile(path, decision.Body, force)
		if err != nil {
			return summary, err
		}
		if wrote {
			summary.Decisions++
		} else {
			summary.SkippedExisting = append(summary.SkippedExisting, relPath(root, path))
		}
	}
	for _, issue := range snap.Issues {
		issueRef := safeSnapshotSegment(issue.Issue, "issue")
		path := filepath.Join(root, DirName, "issues", issueRef+".md")
		state := IssueState{
			Issue:         issueRef,
			Title:         issue.Title,
			Primary:       issue.Primary,
			Related:       issue.Related,
			Status:        issue.Status,
			Owner:         issue.Owner,
			CurrentStage:  issue.CurrentStage,
			CurrentLoop:   issue.CurrentLoop,
			LastResult:    issue.LastResult,
			OpenQuestions: issue.OpenQuestions,
			Blockers:      issue.Blockers,
			NextHandoff:   issue.NextHandoff,
			Audit:         issue.Audit,
			UpdatedAt:     issue.UpdatedAt,
		}
		wrote, err := writeSnapshotFile(path, issueStateTemplate(state), force)
		if err != nil {
			return summary, err
		}
		if wrote {
			summary.Issues++
		} else {
			summary.SkippedExisting = append(summary.SkippedExisting, relPath(root, path))
		}
	}
	sort.Strings(summary.SkippedExisting)
	return summary, nil
}

func readFileEpic(root, epicKey string) (FileEpic, error) {
	epicDir := filepath.Join(root, DirName, "epics", epicKey)
	epic := FileEpic{
		Key:       epicKey,
		Title:     titleCase(strings.ReplaceAll(epicKey, "-", " ")),
		Stability: "draft",
	}
	if doc, ok, err := readFileDocument(root, filepath.Join(epicDir, "00-index.md"), "index"); err != nil {
		return FileEpic{}, err
	} else if ok {
		epic.Title = firstNonEmpty(doc.Title, epic.Title)
		epic.Description = firstParagraph(doc.Body)
		epic.Stability = firstNonEmpty(extractStability(doc.Body), epic.Stability)
		epic.Documents = append(epic.Documents, doc)
	}

	entries, err := os.ReadDir(epicDir)
	if err != nil {
		return FileEpic{}, fmt.Errorf("read epic %s: %w", epicKey, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		module, err := readFileModule(root, epicKey, entry.Name())
		if err != nil {
			return FileEpic{}, err
		}
		epic.Modules = append(epic.Modules, module)
	}
	sort.Slice(epic.Modules, func(i, j int) bool { return epic.Modules[i].Key < epic.Modules[j].Key })
	return epic, nil
}

func epicDocumentPath(root, epicKey string, doc FileDocument) string {
	if doc.DocKind == "index" {
		return filepath.Join(root, DirName, "epics", epicKey, "00-index.md")
	}
	return filepath.Join(root, DirName, "epics", epicKey, safeSnapshotSegment(doc.DocKind, "document")+".md")
}

func moduleDocumentPath(root, epicKey, moduleKey string, doc FileDocument) string {
	if doc.DocKind == "index" {
		return filepath.Join(root, DirName, "epics", epicKey, moduleKey, "00-index.md")
	}
	if file, ok := moduleDocFiles[doc.DocKind]; ok {
		return filepath.Join(root, DirName, "epics", epicKey, moduleKey, file)
	}
	return filepath.Join(root, DirName, "epics", epicKey, moduleKey, safeSnapshotSegment(doc.DocKind, "document")+".md")
}

func decisionPath(root string, decision FileDecision) string {
	if strings.HasPrefix(decision.SourcePath, ".spec/decisions/") && !strings.Contains(decision.SourcePath, "..") {
		return filepath.Join(root, filepath.FromSlash(decision.SourcePath))
	}
	name := slug(decision.Title)
	if name == "" {
		name = "decision"
	}
	return filepath.Join(root, DirName, "decisions", name+".md")
}

func writeSnapshotFile(path, body string, force bool) (bool, error) {
	if !force && fileExists(path) {
		return false, nil
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func safeSnapshotSegment(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "/") || strings.Contains(value, "\\") || value == "." || value == ".." {
		return fallback
	}
	var b strings.Builder
	for _, r := range value {
		if r == 0 || r < 32 {
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

func readFileModule(root, epicKey, moduleKey string) (FileModule, error) {
	moduleDir := filepath.Join(root, DirName, "epics", epicKey, moduleKey)
	module := FileModule{
		Key:       moduleKey,
		Title:     titleCase(strings.ReplaceAll(moduleKey, "-", " ")),
		Stability: "draft",
	}
	if doc, ok, err := readFileDocument(root, filepath.Join(moduleDir, "00-index.md"), "index"); err != nil {
		return FileModule{}, err
	} else if ok {
		module.Title = firstNonEmpty(doc.Title, module.Title)
		module.Description = firstParagraph(doc.Body)
		module.Stability = firstNonEmpty(extractStability(doc.Body), module.Stability)
		module.Documents = append(module.Documents, doc)
	}
	for _, docKind := range sortedDocNames() {
		doc, ok, err := readFileDocument(root, filepath.Join(moduleDir, moduleDocFiles[docKind]), docKind)
		if err != nil {
			return FileModule{}, err
		}
		if ok {
			module.Documents = append(module.Documents, doc)
		}
	}
	return module, nil
}

func readFileDecisions(root string) ([]FileDecision, error) {
	pattern := filepath.Join(root, DirName, "decisions", "*.md")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	sort.Strings(paths)
	out := make([]FileDecision, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read decision %s: %w", path, err)
		}
		body := string(data)
		out = append(out, FileDecision{
			Title:      firstMarkdownHeading(body, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))),
			Body:       body,
			Actor:      "",
			SourcePath: relPath(root, path),
		})
	}
	return out, nil
}

func readFileIssues(root string) ([]FileIssue, []string, error) {
	paths, err := filepath.Glob(filepath.Join(root, DirName, "issues", "*.md"))
	if err != nil {
		return nil, nil, fmt.Errorf("list issue files: %w", err)
	}
	sort.Strings(paths)
	issues := make([]FileIssue, 0, len(paths))
	var skipped []string
	for _, path := range paths {
		issueID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		state, _, err := readIssueState(path, issueID)
		if err != nil {
			skipped = append(skipped, relPath(root, path))
			continue
		}
		issues = append(issues, FileIssue{
			Issue:         firstNonEmpty(state.Issue, issueID),
			Title:         state.Title,
			Primary:       state.Primary,
			Related:       state.Related,
			Status:        state.Status,
			Owner:         state.Owner,
			CurrentStage:  state.CurrentStage,
			CurrentLoop:   state.CurrentLoop,
			LastResult:    state.LastResult,
			OpenQuestions: state.OpenQuestions,
			Blockers:      state.Blockers,
			NextHandoff:   state.NextHandoff,
			Audit:         state.Audit,
			UpdatedAt:     state.UpdatedAt,
			SourcePath:    relPath(root, path),
		})
	}
	sort.Strings(skipped)
	return issues, skipped, nil
}

func readFileDocument(root, path, docKind string) (FileDocument, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return FileDocument{}, false, nil
	}
	if err != nil {
		return FileDocument{}, false, fmt.Errorf("read spec document %s: %w", path, err)
	}
	body := string(data)
	return FileDocument{
		DocKind:    docKind,
		Title:      firstMarkdownHeading(body, titleCase(docKind)),
		Body:       body,
		SourcePath: relPath(root, path),
	}, true, nil
}

func firstMarkdownHeading(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

func firstParagraph(body string) string {
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "#") || strings.HasPrefix(block, "-") {
			continue
		}
		return block
	}
	return ""
}

func extractStability(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "- stability:") && !strings.HasPrefix(lower, "stability:") {
			continue
		}
		_, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "draft" || value == "active" || value == "stable" || value == "deprecated" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
