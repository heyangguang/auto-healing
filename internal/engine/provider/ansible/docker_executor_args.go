package ansible

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildDockerArgs 构建 Docker 命令行参数
func (e *DockerExecutor) buildDockerArgs(req *ExecuteRequest, containerName string) []string {
	args := []string{"run", "--rm", "-t", "--name", containerName, "--network", "host"}
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:ro", req.WorkDir), "-w", "/workspace")
	args = append(args,
		"-e", "ANSIBLE_FORCE_COLOR=0",
		"-e", "ANSIBLE_NOCOLOR=1",
		"-e", "ANSIBLE_PYTHON_INTERPRETER=auto",
		"-e", "PYTHONUNBUFFERED=1",
		"-e", "ANSIBLE_INTERPRETER_PYTHON_FALLBACK=python3.11,python3.10,python3.9,python3.8,python3.7,python3.6,/usr/bin/python3,/usr/libexec/platform-python,python2.7,/usr/bin/python,python",
	)
	args = append(args, e.image, "/workspace/"+req.PlaybookPath)
	args = append(args, dockerInventoryArgs(req)...)
	if len(req.ExtraVars) > 0 {
		jsonVars, _ := json.Marshal(req.ExtraVars)
		args = append(args, "--extra-vars", string(jsonVars))
	}
	if req.Limit != "" {
		args = append(args, "--limit", req.Limit)
	}
	if len(req.Tags) > 0 {
		args = append(args, "--tags", strings.Join(req.Tags, ","))
	}
	if len(req.SkipTags) > 0 {
		args = append(args, "--skip-tags", strings.Join(req.SkipTags, ","))
	}
	if req.Verbosity > 0 {
		args = append(args, buildVerbosityFlag(req.Verbosity))
	}
	if req.Become {
		args = append(args, "--become")
		if req.BecomeUser != "" {
			args = append(args, "--become-user", req.BecomeUser)
		}
	}
	return args
}

func dockerInventoryArgs(req *ExecuteRequest) []string {
	if req.Inventory == "" {
		return nil
	}
	if strings.HasPrefix(req.Inventory, req.WorkDir) {
		return []string{"-i", "/workspace" + strings.TrimPrefix(req.Inventory, req.WorkDir)}
	}
	if strings.Contains(req.Inventory, " ") || strings.Contains(req.Inventory, "\n") {
		inventoryPath := filepath.Join(req.WorkDir, "inventory.ini")
		if err := os.WriteFile(inventoryPath, []byte("[all]\n"+req.Inventory+"\n"), 0644); err == nil {
			return []string{"-i", "/workspace/inventory.ini"}
		}
		return []string{"-i", req.Inventory + ","}
	}
	return []string{"-i", req.Inventory + ","}
}

// CheckDockerInstalled 检查 Docker 是否已安装
func (e *DockerExecutor) CheckDockerInstalled() error {
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not found: %w", err)
	}
	return nil
}

// CheckImageExists 检查镜像是否存在
func (e *DockerExecutor) CheckImageExists() error {
	cmd := exec.Command("docker", "image", "inspect", e.image)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("image %s not found: %w", e.image, err)
	}
	return nil
}
