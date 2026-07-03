package issuedraft

import (
	"errors"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPrimaryRepositoryFromResource(t *testing.T) {
	resource := db.ProjectResource{
		ID:           util.MustParseUUID("00000000-0000-0000-0000-000000000001"),
		ResourceType: localDirectoryResourceType,
		ResourceRef:  []byte(`{"local_path":"/Users/example/lms-mini","daemon_id":"daemon-1","label":"mini"}`),
	}

	repo, err := primaryRepositoryFromResource(resource)
	if err != nil {
		t.Fatalf("primaryRepositoryFromResource() error = %v", err)
	}
	if repo.LocalPath != "/Users/example/lms-mini" {
		t.Fatalf("LocalPath = %q", repo.LocalPath)
	}
	if repo.DaemonID != "daemon-1" {
		t.Fatalf("DaemonID = %q", repo.DaemonID)
	}
	if repo.Label != "mini" {
		t.Fatalf("Label = %q", repo.Label)
	}
}

func TestPrimaryRepositoryFromResourceRejectsMissingPath(t *testing.T) {
	_, err := primaryRepositoryFromResource(db.ProjectResource{
		ResourceType: localDirectoryResourceType,
		ResourceRef:  []byte(`{"daemon_id":"daemon-1"}`),
	})
	if !errors.Is(err, ErrInvalidProjectResource) {
		t.Fatalf("error = %v, want ErrInvalidProjectResource", err)
	}
}

func TestRenderDetailedSpecIncludesImplementationPlan(t *testing.T) {
	out := RenderDetailedSpec(TemplateInput{
		Title:              "创建 issue 时选择小队",
		ImplementationPlan: "1. 添加 issue draft session\n2. 接入确认后写 .spec",
		AcceptanceCriteria: []string{"可以选择小队", "确认后写入 .spec"},
	})

	for _, want := range []string{
		"# 创建 issue 时选择小队",
		"## 10. 实施任务拆分",
		"1. 添加 issue draft session",
		"- [ ] 可以选择小队",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderDetailedSpec() missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderRemoteIssueOmitsImplementationPlan(t *testing.T) {
	out := RenderRemoteIssue(TemplateInput{
		Goal:               "创建 issue 前先和小队澄清需求",
		ImplementationPlan: "这里是详细实施计划，不应该进入远端 issue",
		AcceptanceCriteria: []string{"远端 issue 保持精简"},
	})

	if strings.Contains(out, "这里是详细实施计划") {
		t.Fatalf("RenderRemoteIssue() leaked detailed implementation plan:\n%s", out)
	}
	if !strings.Contains(out, "The detailed implementation plan is stored in the target repository .spec file") {
		t.Fatalf("RenderRemoteIssue() missing .spec pointer:\n%s", out)
	}
}
