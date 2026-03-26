package playbook

import (
	"os"
	"path/filepath"
	"strings"
)

func (vs *VariableScanner) scanIncludes(data interface{}, currentFile string) {
	currentDir := filepath.Dir(currentFile)

	switch value := data.(type) {
	case []interface{}:
		for _, item := range value {
			vs.scanIncludes(item, currentFile)
		}
	case map[string]interface{}:
		vs.scanIncludedFiles(value, currentDir)
		vs.scanRoles(value["roles"], currentFile)
		vs.scanVarsFiles(value["vars_files"], currentFile)
		vs.scanTemplates(value, currentFile)
		for _, item := range value {
			vs.scanIncludes(item, currentFile)
		}
	}
}

func (vs *VariableScanner) scanIncludedFiles(data map[string]interface{}, currentDir string) {
	for _, key := range []string{"include_tasks", "import_tasks", "include", "import_playbook"} {
		path, ok := data[key].(string)
		if !ok {
			continue
		}
		includePath := filepath.Join(currentDir, path)
		if _, err := os.Stat(includePath); err == nil {
			vs.ScanFile(includePath)
		}
	}
}

func (vs *VariableScanner) scanRoles(raw any, currentFile string) {
	roles, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, role := range roles {
		vs.scanRole(role, currentFile)
	}
}

func (vs *VariableScanner) scanVarsFiles(raw any, currentFile string) {
	varsFiles, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, item := range varsFiles {
		path, ok := item.(string)
		if ok {
			vs.scanVarsFile(path, currentFile)
		}
	}
}

func (vs *VariableScanner) scanTemplates(data map[string]interface{}, currentFile string) {
	if template, ok := data["template"].(map[string]interface{}); ok {
		if src, ok := template["src"].(string); ok {
			vs.scanTemplateFile(src, currentFile)
		}
	}
	if src, ok := data["template"].(string); ok {
		vs.scanTemplateFile(src, currentFile)
	}
}

func (vs *VariableScanner) scanRole(role interface{}, _ string) {
	roleName := resolveRoleName(role)
	if roleName == "" {
		return
	}

	roleBase := filepath.Join(vs.basePath, "roles", roleName)
	for _, dir := range []string{"tasks", "handlers", "vars", "defaults", "files", "templates", "meta"} {
		vs.scanRoleDirectory(filepath.Join(roleBase, dir))
	}
}

func resolveRoleName(role interface{}) string {
	switch value := role.(type) {
	case string:
		return value
	case map[string]interface{}:
		if name, ok := value["role"].(string); ok {
			return name
		}
		if name, ok := value["name"].(string); ok {
			return name
		}
	}
	return ""
}

func (vs *VariableScanner) scanRoleDirectory(dirPath string) {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dirPath, entry.Name())
		switch {
		case strings.HasSuffix(entry.Name(), ".yml"), strings.HasSuffix(entry.Name(), ".yaml"):
			vs.ScanFile(fullPath)
		case strings.HasSuffix(entry.Name(), ".j2"):
			vs.scanJinja2File(fullPath)
		}
	}
}

func (vs *VariableScanner) scanVarsFile(path string, currentFile string) {
	currentDir := filepath.Dir(currentFile)
	if strings.Contains(path, "{{") && strings.Contains(path, "}}") {
		vs.scanDynamicVarsDir(filepath.Join(currentDir, filepath.Dir(path)))
		return
	}

	varsPath := filepath.Join(currentDir, path)
	if _, err := os.Stat(varsPath); err == nil {
		vs.ScanFile(varsPath)
	}
}

func (vs *VariableScanner) scanDynamicVarsDir(varsDir string) {
	info, err := os.Stat(varsDir)
	if err != nil || !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(varsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") {
			vs.ScanFile(filepath.Join(varsDir, entry.Name()))
		}
	}
}

func (vs *VariableScanner) scanTemplateFile(src string, currentFile string) {
	for _, templatePath := range vs.templateSearchPaths(src, currentFile) {
		if _, err := os.Stat(templatePath); err == nil {
			vs.scanJinja2File(templatePath)
			return
		}
	}
}

func (vs *VariableScanner) templateSearchPaths(src string, currentFile string) []string {
	currentDir := filepath.Dir(currentFile)
	searchPaths := []string{
		filepath.Join(currentDir, src),
		filepath.Join(currentDir, "templates", src),
		filepath.Join(vs.basePath, "templates", src),
	}

	if !strings.Contains(currentFile, "/roles/") {
		return searchPaths
	}
	parts := strings.Split(currentFile, "/roles/")
	if len(parts) <= 1 {
		return searchPaths
	}
	roleParts := strings.SplitN(parts[1], "/", 2)
	if len(roleParts) == 0 {
		return searchPaths
	}
	return append(searchPaths, filepath.Join(parts[0], "roles", roleParts[0], "templates", src))
}

func (vs *VariableScanner) scanJinja2File(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	if vs.scannedFiles[absPath] {
		return
	}
	vs.scannedFiles[absPath] = true

	content, err := os.ReadFile(filePath)
	if err == nil {
		vs.scanVariableReferences(string(content), filePath)
	}
}

func isBuiltinVariable(name string) bool {
	builtins := map[string]bool{
		"item":               true,
		"ansible_facts":      true,
		"ansible_host":       true,
		"ansible_user":       true,
		"ansible_password":   true,
		"inventory_hostname": true,
		"hostvars":           true,
		"groups":             true,
		"group_names":        true,
		"play_hosts":         true,
		"ansible_play_hosts": true,
		"ansible_check_mode": true,
		"ansible_version":    true,
		"ansible_date_time":  true,
		"ansible_env":        true,
		"ansible_connection": true,
		"ansible_ssh_host":   true,
		"lookup":             true,
		"omit":               true,
		"now":                true,
		"true":               true,
		"false":              true,
	}
	for _, prefix := range []string{"ansible_", "hostvars", "groups"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return builtins[name]
}
