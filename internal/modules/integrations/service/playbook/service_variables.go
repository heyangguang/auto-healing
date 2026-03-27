package playbook

import "github.com/company/auto-healing/internal/modules/integrations/model"

func (s *Service) mergeVariables(userVars, scannedVars model.JSONArray) model.JSONArray {
	userVarMap := indexVariablesByName(userVars)
	scannedNameSet := buildVariableNameSet(scannedVars)
	result := make(model.JSONArray, 0, len(userVarMap)+len(scannedVars))

	for _, variable := range scannedVars {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, _ := vm["name"].(string)
		result = append(result, mergeVariable(name, vm, userVarMap[name]))
	}

	for name, userVar := range userVarMap {
		if !scannedNameSet[name] {
			userVar["in_code"] = false
			result = append(result, userVar)
		}
	}
	return result
}

func indexVariablesByName(vars model.JSONArray) map[string]map[string]any {
	result := make(map[string]map[string]any, len(vars))
	for _, variable := range vars {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, ok := vm["name"].(string)
		if ok {
			result[name] = vm
		}
	}
	return result
}

func buildVariableNameSet(vars model.JSONArray) map[string]bool {
	result := make(map[string]bool, len(vars))
	for _, variable := range vars {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, ok := vm["name"].(string)
		if ok {
			result[name] = true
		}
	}
	return result
}

func mergeVariable(name string, scanned map[string]any, user map[string]any) map[string]any {
	if user == nil {
		scanned["in_code"] = true
		return scanned
	}

	merged := cloneVariableMap(user)
	merged["sources"] = scanned["sources"]
	merged["primary_source"] = scanned["primary_source"]
	merged["in_code"] = true
	merged["type_source"] = scanned["type_source"]

	if typeSource, _ := scanned["type_source"].(string); typeSource == "enhanced" {
		if newType, ok := scanned["type"].(string); ok {
			if oldType, _ := merged["type"].(string); oldType != newType {
				merged["type"] = newType
				if newDefault := scanned["default"]; newDefault != nil {
					merged["default"] = newDefault
				}
			}
		}
	}

	return merged
}

func cloneVariableMap(src map[string]any) map[string]any {
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func (s *Service) countChanges(oldVars, newVars model.JSONArray) (newCount, removedCount int) {
	oldNames := buildVariableNameSet(oldVars)
	newNames := buildVariableNameSet(newVars)

	for name := range newNames {
		if !oldNames[name] {
			newCount++
		}
	}
	for name := range oldNames {
		if !newNames[name] {
			removedCount++
		}
	}
	return
}

func getMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func getNewVariableNames(oldVars, newVars model.JSONArray) []string {
	oldNames := buildVariableNameSet(oldVars)
	result := make([]string, 0, len(newVars))
	for _, variable := range newVars {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, ok := vm["name"].(string)
		if ok && !oldNames[name] {
			result = append(result, name)
		}
	}
	return result
}

func getRemovedVariableNames(oldVars, newVars model.JSONArray) []string {
	newNames := buildVariableNameSet(newVars)
	result := make([]string, 0, len(oldVars))
	for _, variable := range oldVars {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, ok := vm["name"].(string)
		if ok && !newNames[name] {
			result = append(result, name)
		}
	}
	return result
}
