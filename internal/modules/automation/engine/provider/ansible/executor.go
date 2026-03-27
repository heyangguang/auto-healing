package ansible

import (
	"context"
	"time"
)

// Executor 执行器接口
type Executor interface {
	// Execute 执行 Ansible Playbook
	Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error)
	// Name 返回执行器名称
	Name() string
}

// LogCallback 日志回调函数类型
type LogCallback func(level, stage, message string)

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	PlaybookPath string         // playbook 相对路径 (相对于工作目录)
	WorkDir      string         // 工作目录 (临时目录)
	Inventory    string         // Inventory 内容或文件路径
	ExtraVars    map[string]any // --extra-vars
	Limit        string         // --limit 限制目标主机
	Tags         []string       // --tags
	SkipTags     []string       // --skip-tags
	Verbosity    int            // -v 级别 (0-4)
	Timeout      time.Duration  // 执行超时
	Become       bool           // --become
	BecomeUser   string         // --become-user
	LogCallback  LogCallback    // 日志回调（用于实时流式输出）
}

// ExecuteResult 执行结果
type ExecuteResult struct {
	ExitCode  int           // 退出码
	Stdout    string        // 标准输出
	Stderr    string        // 标准错误
	Stats     *AnsibleStats // 统计信息
	StartedAt time.Time     // 开始时间
	Duration  time.Duration // 执行时长
}

// AnsibleStats Ansible 执行统计
type AnsibleStats struct {
	Ok          int            `json:"ok"`
	Changed     int            `json:"changed"`
	Unreachable int            `json:"unreachable"`
	Failed      int            `json:"failed"`
	Skipped     int            `json:"skipped"`
	Rescued     int            `json:"rescued"`
	Ignored     int            `json:"ignored"`
	HostStats   map[string]int `json:"host_stats,omitempty"` // 每个主机的统计
}

// IsSuccess 检查执行是否成功
func (s *AnsibleStats) IsSuccess() bool {
	return s.Failed == 0 && s.Unreachable == 0
}

// TotalHosts 返回涉及的主机总数
func (s *AnsibleStats) TotalHosts() int {
	return len(s.HostStats)
}
