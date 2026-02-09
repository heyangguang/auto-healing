package ansible

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// PLAY RECAP *********************************************************************
	// localhost                  : ok=2    changed=1    unreachable=0    failed=0    skipped=0    rescued=0    ignored=0
	recapLineRegex = regexp.MustCompile(`^(\S+)\s*:\s*ok=(\d+)\s+changed=(\d+)\s+unreachable=(\d+)\s+failed=(\d+)\s+skipped=(\d+)(?:\s+rescued=(\d+))?(?:\s+ignored=(\d+))?`)
)

// ParseStats 从 Ansible 输出解析 PLAY RECAP 统计信息
func ParseStats(output string) *AnsibleStats {
	stats := &AnsibleStats{
		HostStats: make(map[string]int),
	}

	lines := strings.Split(output, "\n")
	inRecap := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 检测 PLAY RECAP 开始
		if strings.Contains(line, "PLAY RECAP") {
			inRecap = true
			continue
		}

		if !inRecap {
			continue
		}

		// 解析主机统计行
		matches := recapLineRegex.FindStringSubmatch(line)
		if len(matches) >= 6 {
			host := matches[1]
			ok, _ := strconv.Atoi(matches[2])
			changed, _ := strconv.Atoi(matches[3])
			unreachable, _ := strconv.Atoi(matches[4])
			failed, _ := strconv.Atoi(matches[5])
			skipped, _ := strconv.Atoi(matches[6])

			stats.Ok += ok
			stats.Changed += changed
			stats.Unreachable += unreachable
			stats.Failed += failed
			stats.Skipped += skipped

			if len(matches) >= 8 && matches[7] != "" {
				rescued, _ := strconv.Atoi(matches[7])
				stats.Rescued += rescued
			}
			if len(matches) >= 9 && matches[8] != "" {
				ignored, _ := strconv.Atoi(matches[8])
				stats.Ignored += ignored
			}

			// 记录每个主机的 failed 数
			stats.HostStats[host] = failed
		}
	}

	return stats
}

// ParseTaskOutput 解析任务级输出 (可选, 用于详细日志)
type TaskOutput struct {
	PlayName  string
	TaskName  string
	Host      string
	Status    string // ok, changed, failed, skipped, unreachable
	Message   string
	StartTime string
	Duration  string
}

// ParseTasksFromOutput 从输出解析任务列表 (简化版)
func ParseTasksFromOutput(output string) []TaskOutput {
	var tasks []TaskOutput

	// 简化实现: 只匹配 TASK [xxx] 行
	taskRegex := regexp.MustCompile(`TASK \[([^\]]+)\]`)
	okRegex := regexp.MustCompile(`ok: \[([^\]]+)\]`)
	changedRegex := regexp.MustCompile(`changed: \[([^\]]+)\]`)
	failedRegex := regexp.MustCompile(`fatal: \[([^\]]+)\]`)

	lines := strings.Split(output, "\n")
	currentTask := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if matches := taskRegex.FindStringSubmatch(line); len(matches) > 1 {
			currentTask = matches[1]
			continue
		}

		if currentTask == "" {
			continue
		}

		var host, status string
		if matches := okRegex.FindStringSubmatch(line); len(matches) > 1 {
			host = matches[1]
			status = "ok"
		} else if matches := changedRegex.FindStringSubmatch(line); len(matches) > 1 {
			host = matches[1]
			status = "changed"
		} else if matches := failedRegex.FindStringSubmatch(line); len(matches) > 1 {
			host = matches[1]
			status = "failed"
		}

		if host != "" {
			tasks = append(tasks, TaskOutput{
				TaskName: currentTask,
				Host:     host,
				Status:   status,
			})
		}
	}

	return tasks
}
