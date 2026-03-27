package secrets

import (
	"encoding/json"

	"github.com/company/auto-healing/internal/platform/modeltypes"
)

func jsonEqual(a, b modeltypes.JSON) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}
