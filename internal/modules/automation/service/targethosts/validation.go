package targethosts

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
)

// ValidateActiveCMDBHosts ensures every execution target exists as an active CMDB item.
func ValidateActiveCMDBHosts(ctx context.Context, repo *cmdbrepo.CMDBItemRepository, targetHosts string) error {
	hosts := Parse(targetHosts)
	if len(hosts) == 0 {
		return fmt.Errorf("目标主机不能为空")
	}
	if repo == nil {
		return nil
	}

	missing := make([]string, 0)
	for _, host := range hosts {
		if _, err := repo.FindActiveByNameOrIP(ctx, host); err != nil {
			if errors.Is(err, cmdbrepo.ErrCMDBItemNotFound) {
				missing = append(missing, host)
				continue
			}
			return fmt.Errorf("校验目标主机 %s 失败: %w", host, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("目标主机未在 CMDB 中登记或不是 active 状态: %s", strings.Join(missing, ", "))
	}
	return nil
}

func Parse(targetHosts string) []string {
	parts := strings.FieldsFunc(targetHosts, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})

	seen := make(map[string]bool, len(parts))
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		host := normalizeHost(part)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		hosts = append(hosts, host)
	}
	return hosts
}

func normalizeHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(splitHost, "[]")
	}

	host = strings.Trim(host, "[]")
	if idx := strings.LastIndex(host, ":"); idx > 0 && strings.Count(host, ":") == 1 {
		if isNumericPort(host[idx+1:]) {
			return host[:idx]
		}
	}
	return host
}

func isNumericPort(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
