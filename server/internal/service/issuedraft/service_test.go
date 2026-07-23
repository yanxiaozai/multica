package issuedraft

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

func TestBuildTemplateInputSummarizesConversationIntoIssue(t *testing.T) {
	in := buildTemplateInput(
		db.IssueDraftSession{
			PrimaryLocalPathSnapshot: "/Users/example/lms-mini",
			CreatedAt:                pgtype.Timestamptz{Time: time.Date(2026, 7, 7, 18, 16, 0, 0, time.UTC), Valid: true},
		},
		[]db.IssueDraftMessage{
			{AuthorType: AuthorMember, Content: "预约「立即预约创建流程」+修改/"},
			{AuthorType: AuthorMember, Content: "1.新增“立即预约创建流程”\n2.预约概览\n3.需要查看work-module的接口"},
			{AuthorType: AuthorMember, Content: "1.客户列表页。`memberId`、客户姓名、手机号、会员等级都要\n2.如果来源是预约概览，要求下次进入日历自动刷新即可"},
		},
		[]db.IssueDraftMemberTask{
			{Findings: "work-contact 存在预约创建接口，状态 WAIT_USED 表示已使用。"},
		},
	)

	out := RenderMulticaIssue(in)
	for _, forbidden := range []string{
		"用户原始需求:",
		"- 1.新增",
		"- 1.客户列表页",
		"小队成员发现:",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("RenderMulticaIssue() leaked raw conversation marker %q in:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{
		"已澄清的需求要点",
		"立即预约创建流程",
		"客户字段包含 memberId、姓名、手机号、会员等级等必要信息",
		"相关视图刷新",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderMulticaIssue() missing summarized issue content %q in:\n%s", want, out)
		}
	}
}
