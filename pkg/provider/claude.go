package provider

import "github.com/yang-bin-free/claude-phone/pkg/session"

type ClaudeAdapter struct {
	bin string
}

func NewClaudeAdapter(bin string) *ClaudeAdapter {
	if bin == "" {
		bin = "claude"
	}
	return &ClaudeAdapter{bin: bin}
}

func (a *ClaudeAdapter) Descriptor() Descriptor {
	return Descriptor{
		ID: ClaudeID, Name: "Claude Code", Available: true,
		Permissions: []PermissionOption{
			{ID: "default", Label: "每次询问", Description: "修改文件或执行受限操作前征求同意。", Mutable: true},
			{ID: "acceptEdits", Label: "自动接受编辑", Description: "自动接受文件编辑，其他危险操作仍会询问。", Mutable: true},
			{ID: "plan", Label: "只读规划", Description: "分析和制定计划，不修改源代码。", Mutable: true},
			{ID: "bypassPermissions", Label: "完全访问", Description: "跳过常规权限限制。", Dangerous: true, Mutable: true},
		},
	}
}

func (a *ClaudeAdapter) NewProcess(config SessionConfig) Process {
	return session.NewClaudeProc(session.ClaudeConfig{
		Bin: a.bin, Cwd: config.Cwd, SessionID: config.SessionID,
		Permission: config.Permission, AddDirs: config.AddDirs, Resume: config.Resume,
		AllowedTools: config.AllowedTools,
	})
}
