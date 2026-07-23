package issuedraft

import (
	"fmt"
	"strings"
	"time"
)

type TemplateInput struct {
	Title                string
	Background           string
	DuplicateSearch      string
	SplitDecision        string
	Goal                 string
	NonGoal              string
	ImpactScope          string
	UserScenario         string
	AcceptanceCriteria   []string
	TechnicalConstraints string
	DesignRecord         string
	ImplementationPlan   string
	VerificationPlan     string
	RisksAndRollback     string
	RelatedInfo          string
	PMAgent              string
	ArchitectAgent       string
	PMCreatedAt          time.Time
	ArchitectAlignedAt   time.Time
}

func RenderDetailedSpec(in TemplateInput) string {
	return strings.TrimSpace(fmt.Sprintf(`# %s

## 1. 背景

负责人: PM

%s

## 2. 历史查询记录

负责人: PM

%s

### 2.1 拆分判断

负责人: PM

%s

## 3. 目标

负责人: PM

%s

## 4. 非目标

负责人: PM

%s

## 5. 影响范围

负责人: PM + ARCH

%s

## 6. 用户场景

负责人: PM

%s

## 7. 验收标准

负责人: PM

%s

## 8. 技术约束

负责人: ARCH

%s

## 9. 设计与方案记录

负责人: ARCH

%s

## 10. 实施任务拆分

负责人: ARCH

%s

## 11. 验证计划

负责人: ARCH

%s

## 12. 风险与回滚

负责人: ARCH

%s

## 13. 关联信息

负责人: PM

%s

## 14. Agent 工作记录

负责人: PM + ARCH

- PM: %s
- ARCH: %s
- PM 创建时间: %s
- ARCH 接口对齐时间: %s
`,
		fallback(in.Title, "Untitled issue"),
		fallback(in.Background, "TBD"),
		fallback(in.DuplicateSearch, "TBD"),
		fallback(in.SplitDecision, "TBD"),
		fallback(in.Goal, "TBD"),
		fallback(in.NonGoal, "TBD"),
		fallback(in.ImpactScope, "TBD"),
		fallback(in.UserScenario, "TBD"),
		renderChecklist(in.AcceptanceCriteria),
		fallback(in.TechnicalConstraints, "TBD"),
		fallback(in.DesignRecord, "TBD"),
		fallback(in.ImplementationPlan, "TBD"),
		fallback(in.VerificationPlan, "TBD"),
		fallback(in.RisksAndRollback, "TBD"),
		fallback(in.RelatedInfo, "无"),
		fallback(in.PMAgent, "TBD"),
		fallback(in.ArchitectAgent, "TBD"),
		renderTime(in.PMCreatedAt),
		renderTime(in.ArchitectAlignedAt),
	))
}

func RenderMulticaIssue(in TemplateInput) string {
	return strings.TrimSpace(fmt.Sprintf(`## 背景

%s

## 目标

%s

## 验收标准

%s

## 关联信息

%s
`,
		fallback(in.Background, "TBD"),
		fallback(in.Goal, "TBD"),
		renderChecklist(in.AcceptanceCriteria),
		fallback(in.RelatedInfo, "无"),
	))
}

func RenderRemoteIssue(in TemplateInput) string {
	return strings.TrimSpace(fmt.Sprintf(`## Summary

%s

## Acceptance Criteria

%s

## Reference

The detailed implementation plan is stored in the target repository .spec file after confirmation.
`,
		fallback(in.Goal, fallback(in.Background, "TBD")),
		renderChecklist(in.AcceptanceCriteria),
	))
}

func renderChecklist(items []string) string {
	if len(items) == 0 {
		return "- [ ] TBD"
	}
	var b strings.Builder
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- [ ] ")
		b.WriteString(item)
		b.WriteByte('\n')
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "- [ ] TBD"
	}
	return out
}

func fallback(value, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func renderTime(t time.Time) string {
	if t.IsZero() {
		return "TBD"
	}
	return t.Format(time.RFC3339)
}
