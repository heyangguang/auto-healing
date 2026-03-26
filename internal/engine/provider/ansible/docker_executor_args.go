package ansible

import (
	"fmt"
	"os/exec"
	"strings"
)

// buildDockerArgs 构建 Docker 命令行参数
func (e *DockerExecutor) buildDockerArgs(req *ExecuteRequest, containerName string) ([]string, error) {
	playbookPath, err := resolveContainerPathWithinWorkDir(req.WorkDir, req.PlaybookPath, "playbook")
	if err != nil {
		return nil, err
	}
	args := []string{"run", "--rm", "--name", containerName, "--network", "host"}
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:ro", req.WorkDir), "-w", "/workspace")
	args = append(args,
		"-e", "ANSIBLE_FORCE_COLOR=0",
		"-e", "ANSIBLE_NOCOLOR=1",
		"-e", "ANSIBLE_PYTHON_INTERPRETER=auto",
		"-e", "PYTHONUNBUFFERED=1",
		"-e", "ANSIBLE_INTERPRETER_PYTHON_FALLBACK=python3.11,python3.10,python3.9,python3.8,python3.7,python3.6,/usr/bin/python3,/usr/libexec/platform-python,python2.7,/usr/bin/python,python",
	)
	args = append(args, e.image, playbookPath)
	inventoryArgs, err := dockerInventoryArgs(req)
	if err != nil {
		return nil, err
	}
	args = append(args, inventoryArgs...)
	if len(req.ExtraVars) > 0 {
		jsonVars, err := marshalExtraVars(req.ExtraVars)
		if err != nil {
			return nil, err
		}
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
	return args, nil
}

func dockerInventoryArgs(req *ExecuteRequest) ([]string, error) {
	if req.Inventory == "" {
		return nil, nil
	}
	path, err := resolveInventoryFile(req.WorkDir, req.Inventory)
	if err != nil {
		return nil, err
	}
	if path != "" {
		return dockerInventoryFileArgs(req.WorkDir, path)
	}
	if strings.Contains(req.Inventory, " ") || strings.Contains(req.Inventory, "\n") {
		path, err = WriteInventoryFile(req.WorkDir, "[all]\n"+req.Inventory+"\n")
		if err != nil {
			return nil, fmt.Errorf("写入 docker inventory 文件失败: %w", err)
		}
		return dockerInventoryFileArgs(req.WorkDir, path)
	}
	return []string{"-i", req.Inventory + ","}, nil
}

func dockerInventoryFileArgs(workDir, path string) ([]string, error) {
	relPath, err := relativePathWithinWorkDir(workDir, path)
	if err != nil {
		return nil, err
	}
	return []string{"-i", "/workspace/" + strings.TrimPrefix(relPath, "./")}, nil
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
