package provider

import (
	"fmt"
	"net/url"
	"strings"
)

func validateProviderURL(rawURL, fieldName string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return newConfigError(fmt.Sprintf("%s 无效", fieldName))
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return newConfigError(fmt.Sprintf("%s 无效", fieldName))
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return newConfigError(fmt.Sprintf("%s 只支持 http 或 https", fieldName))
	}
	return nil
}
