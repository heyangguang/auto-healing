package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func newJSONRequest(
	ctx context.Context,
	requestURL string,
	payload interface{},
) (*http.Request, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}
