package playbook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScannedVariable 扫描到的变量
type ScannedVariable struct {
	Name          string
	Type          string
	Required      bool
	Default       any
	Description   string
	Sources       []VariableSource
	PrimarySource string
	HasDefault    bool
}

// VariableSource 变量来源位置
type VariableSource struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// VariableScanner 变量扫描器（完全递归）
type VariableScanner struct {
	basePath     string
	scannedState map[string]bool
	scannedFiles map[string]bool
	variables    map[string]*ScannedVariable
	err          error
}

// ScanFile 扫描文件（递归）
func (vs *VariableScanner) ScanFile(filePath string) error {
	return vs.scanFileWithContext(filePath, map[string]any{})
}

func (vs *VariableScanner) scanFileWithContext(filePath string, currentVars map[string]any) error {
	if vs.scannedState == nil {
		vs.scannedState = make(map[string]bool)
	}
	if vs.scannedFiles == nil {
		vs.scannedFiles = make(map[string]bool)
	}
	if vs.variables == nil {
		vs.variables = make(map[string]*ScannedVariable)
	}

	resolvedPath, err := resolveExistingRepoPath(vs.basePath, filePath)
	if err != nil {
		return err
	}
	scanKey := buildScanStateKey(resolvedPath, currentVars)
	if vs.scannedState[scanKey] {
		return nil
	}
	vs.scannedState[scanKey] = true
	vs.scannedFiles[resolvedPath] = true

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return err
	}

	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		vs.scanVariableReferences(string(content), resolvedPath)
		return nil
	}

	vs.scanYAMLStructure(data, resolvedPath)
	vs.scanVariableReferences(string(content), resolvedPath)
	vs.scanIncludes(data, resolvedPath, currentVars)
	return vs.err
}

func buildScanStateKey(path string, currentVars map[string]any) string {
	if len(currentVars) == 0 {
		return path
	}
	keys := make([]string, 0, len(currentVars))
	for key := range currentVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	parts = append(parts, path)
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprint(currentVars[key]))
	}
	return strings.Join(parts, "|")
}

func (vs *VariableScanner) scanYAMLStructure(data interface{}, filePath string) {
	relPath, _ := filepath.Rel(vs.basePath, filePath)

	switch value := data.(type) {
	case []interface{}:
		for _, item := range value {
			vs.scanYAMLStructure(item, filePath)
		}
	case map[string]interface{}:
		vs.scanVariableMap(relPath, value["vars"])
		vs.scanVariableMap(relPath, value["set_fact"])
		for _, item := range value {
			vs.scanYAMLStructure(item, filePath)
		}
	}
}

func (vs *VariableScanner) scanVariableMap(relPath string, raw any) {
	vars, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	for name, value := range vars {
		vs.addVariable(name, value, relPath, 0)
	}
}

func (vs *VariableScanner) scanVariableReferences(content string, filePath string) {
	relPath, _ := filepath.Rel(vs.basePath, filePath)
	re := regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)(\s*\|[^}]*)?\s*\}\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if isBuiltinVariable(name) {
			continue
		}
		vs.addVariable(name, buildDefaultExpression(name, match), relPath, 0)
	}
}

func buildDefaultExpression(name string, match []string) any {
	if len(match) < 3 || match[2] == "" {
		return nil
	}
	return "{{" + name + match[2] + "}}"
}

func (vs *VariableScanner) addVariable(name string, defaultValue any, sourceFile string, sourceLine int) {
	if existing, exists := vs.variables[name]; exists {
		existing.Sources = append(existing.Sources, VariableSource{File: sourceFile, Line: sourceLine})
		vs.updateVariableTypeFromDefault(existing, defaultValue, sourceFile)
		return
	}

	vs.variables[name] = &ScannedVariable{
		Name:          name,
		Type:          inferTypeSmartly(name, defaultValue),
		Default:       defaultValue,
		Sources:       []VariableSource{{File: sourceFile, Line: sourceLine}},
		PrimarySource: sourceFile,
		HasDefault:    hasJinjaDefault(defaultValue),
	}
}

func (vs *VariableScanner) updateVariableTypeFromDefault(variable *ScannedVariable, defaultValue any, sourceFile string) {
	strVal, ok := defaultValue.(string)
	if !ok {
		return
	}
	newType := parseJinja2Default(strVal)
	if newType == "" || variable.Type == newType {
		return
	}

	variable.Type = newType
	variable.Default = defaultValue
	variable.PrimarySource = sourceFile
	variable.HasDefault = true
}

func hasJinjaDefault(defaultValue any) bool {
	strVal, ok := defaultValue.(string)
	return ok && parseJinja2Default(strVal) != ""
}
