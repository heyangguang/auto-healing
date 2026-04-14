package playbook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func (vs *VariableScanner) recordScanError(err error) {
	if err != nil && vs.err == nil {
		vs.err = err
	}
}

func (vs *VariableScanner) scanNestedYAML(path string, currentVars map[string]any) {
	if vs.err != nil {
		return
	}
	if err := vs.scanFileWithContext(path, currentVars); err != nil {
		vs.recordScanError(err)
	}
}

func (vs *VariableScanner) scanNestedTemplate(path string) {
	if vs.err != nil {
		return
	}
	if err := vs.scanJinja2File(path); err != nil {
		vs.recordScanError(err)
	}
}

func (vs *VariableScanner) scanIncludes(data interface{}, currentFile string, currentVars map[string]any) {
	currentDir := filepath.Dir(currentFile)

	switch value := data.(type) {
	case []interface{}:
		for _, item := range value {
			vs.scanIncludes(item, currentFile, currentVars)
		}
	case map[string]interface{}:
		localVars := mergeExecutionContext(currentVars, value["vars"])
		if !shouldTraverseExecutionBlock(value, localVars) {
			return
		}
		vs.scanIncludedFiles(value, currentDir, localVars)
		vs.scanRoles(value["roles"], currentFile, localVars)
		vs.scanRoleReference(value["include_role"], currentFile, localVars)
		vs.scanRoleReference(value["import_role"], currentFile, localVars)
		vs.scanRoleReference(value["ansible.builtin.include_role"], currentFile, localVars)
		vs.scanRoleReference(value["ansible.builtin.import_role"], currentFile, localVars)
		vs.scanVarsFiles(value["vars_files"], currentFile, localVars)
		vs.scanTemplates(value, currentFile)
		for _, item := range value {
			vs.scanIncludes(item, currentFile, localVars)
		}
	}
}

func (vs *VariableScanner) scanIncludedFiles(data map[string]interface{}, currentDir string, currentVars map[string]any) {
	for _, key := range []string{
		"include_tasks",
		"import_tasks",
		"include",
		"import_playbook",
		"ansible.builtin.include_tasks",
		"ansible.builtin.import_tasks",
		"ansible.builtin.import_playbook",
	} {
		path, ok := data[key].(string)
		if !ok {
			continue
		}
		includePath := filepath.Join(currentDir, path)
		if _, err := os.Stat(includePath); err == nil {
			vs.scanNestedYAML(includePath, currentVars)
			if vs.err != nil {
				return
			}
		}
	}
}

func (vs *VariableScanner) scanRoles(raw any, currentFile string, currentVars map[string]any) {
	roles, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, role := range roles {
		vs.scanRole(role, currentFile, currentVars)
	}
}

func (vs *VariableScanner) scanRoleReference(raw any, currentFile string, currentVars map[string]any) {
	if raw == nil {
		return
	}
	vs.scanRole(raw, currentFile, currentVars)
}

func (vs *VariableScanner) scanVarsFiles(raw any, currentFile string, currentVars map[string]any) {
	varsFiles, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, item := range varsFiles {
		path, ok := item.(string)
		if ok {
			vs.scanVarsFile(path, currentFile, currentVars)
		}
	}
}

func (vs *VariableScanner) scanTemplates(data map[string]interface{}, currentFile string) {
	if template, ok := data["template"].(map[string]interface{}); ok {
		if src, ok := template["src"].(string); ok {
			vs.scanTemplateFile(src, currentFile)
		}
	}
	if template, ok := data["ansible.builtin.template"].(map[string]interface{}); ok {
		if src, ok := template["src"].(string); ok {
			vs.scanTemplateFile(src, currentFile)
		}
	}
	if src, ok := data["template"].(string); ok {
		vs.scanTemplateFile(src, currentFile)
	}
}

func (vs *VariableScanner) scanRole(role interface{}, currentFile string, currentVars map[string]any) {
	roleName := resolveRoleName(role)
	if roleName == "" {
		return
	}

	for _, roleBase := range vs.roleSearchPaths(currentFile, roleName) {
		vs.scanRoleEntrypoints(roleBase, currentVars)
	}
}

func (vs *VariableScanner) roleSearchPaths(currentFile, roleName string) []string {
	paths := []string{
		filepath.Join(vs.basePath, "roles", roleName),
		filepath.Join(filepath.Dir(currentFile), "roles", roleName),
	}

	parts := strings.Split(currentFile, string(filepath.Separator)+"roles"+string(filepath.Separator))
	if len(parts) > 1 {
		paths = append(paths, filepath.Join(parts[0], "roles", roleName))
	}

	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		result = append(result, cleaned)
	}
	return result
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

func (vs *VariableScanner) scanRoleEntrypoints(roleBase string, currentVars map[string]any) {
	for _, dir := range []string{"tasks", "handlers", "vars", "defaults", "meta"} {
		vs.scanRoleMainFile(filepath.Join(roleBase, dir), currentVars)
		if vs.err != nil {
			return
		}
	}
	vs.scanRoleAssetDirectory(filepath.Join(roleBase, "templates"))
	vs.scanRoleAssetDirectory(filepath.Join(roleBase, "files"))
}

func (vs *VariableScanner) scanRoleMainFile(dirPath string, currentVars map[string]any) {
	for _, candidate := range []string{"main.yml", "main.yaml"} {
		fullPath := filepath.Join(dirPath, candidate)
		if _, err := os.Stat(fullPath); err == nil {
			vs.scanNestedYAML(fullPath, currentVars)
			return
		}
	}
}

func (vs *VariableScanner) scanRoleAssetDirectory(dirPath string) {
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
		if strings.HasSuffix(entry.Name(), ".j2") {
			vs.scanNestedTemplate(fullPath)
		}
		if vs.err != nil {
			return
		}
	}
}

func (vs *VariableScanner) scanVarsFile(path string, currentFile string, currentVars map[string]any) {
	currentDir := filepath.Dir(currentFile)
	if strings.Contains(path, "{{") && strings.Contains(path, "}}") {
		vs.scanDynamicVarsDir(filepath.Join(currentDir, filepath.Dir(path)), currentVars)
		return
	}

	varsPath := filepath.Join(currentDir, path)
	if _, err := os.Stat(varsPath); err == nil {
		vs.scanNestedYAML(varsPath, currentVars)
	}
}

func (vs *VariableScanner) scanDynamicVarsDir(varsDir string, currentVars map[string]any) {
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
			vs.scanNestedYAML(filepath.Join(varsDir, entry.Name()), currentVars)
			if vs.err != nil {
				return
			}
		}
	}
}

func (vs *VariableScanner) scanTemplateFile(src string, currentFile string) {
	for _, templatePath := range vs.templateSearchPaths(src, currentFile) {
		if _, err := os.Stat(templatePath); err == nil {
			vs.scanNestedTemplate(templatePath)
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

func (vs *VariableScanner) scanJinja2File(filePath string) error {
	resolvedPath, err := resolveExistingRepoPath(vs.basePath, filePath)
	if err != nil {
		return err
	}
	if vs.scannedFiles[resolvedPath] {
		return nil
	}
	vs.scannedFiles[resolvedPath] = true

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return err
	}
	vs.scanVariableReferences(string(content), resolvedPath)
	return nil
}

func mergeExecutionContext(currentVars map[string]any, raw any) map[string]any {
	result := make(map[string]any, len(currentVars))
	for key, value := range currentVars {
		result[key] = value
	}
	varsMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	for key, value := range varsMap {
		if isStaticContextValue(value) {
			result[key] = value
		}
	}
	return result
}

func isStaticContextValue(value any) bool {
	switch value.(type) {
	case string, bool, int, int64, float64:
		return true
	default:
		return false
	}
}

func shouldTraverseExecutionBlock(data map[string]interface{}, currentVars map[string]any) bool {
	whenRaw, exists := data["when"]
	if !exists {
		return true
	}
	return evaluateWhenExpression(whenRaw, currentVars)
}

func evaluateWhenExpression(raw any, currentVars map[string]any) bool {
	switch value := raw.(type) {
	case string:
		return evalWhenClause(value, currentVars)
	case []interface{}:
		for _, item := range value {
			clause, ok := item.(string)
			if !ok {
				continue
			}
			if !evalWhenClause(clause, currentVars) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func evalWhenClause(clause string, currentVars map[string]any) bool {
	expr := strings.TrimSpace(clause)
	if expr == "" {
		return true
	}

	pattern := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*(==|!=)\s*['"]([^'"]+)['"]$`)
	matches := pattern.FindStringSubmatch(expr)
	if len(matches) == 4 {
		left := matches[1]
		operator := matches[2]
		right := matches[3]
		actual, exists := currentVars[left]
		if !exists {
			return true
		}
		actualValue := strings.TrimSpace(fmt.Sprint(actual))
		if operator == "==" {
			return actualValue == right
		}
		return actualValue != right
	}

	if value, exists := currentVars[expr]; exists {
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return typed != ""
		}
	}

	return true
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
