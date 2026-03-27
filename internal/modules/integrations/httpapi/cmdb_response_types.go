package httpapi

type batchConnectionTestResponse struct {
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Failed  int                    `json:"failed"`
	Results []ConnectionTestResult `json:"results"`
}
