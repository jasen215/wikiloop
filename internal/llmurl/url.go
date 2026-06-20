package llmurl

import "strings"

// Endpoint joins an LLM provider base URL with a versioned API path.
// It accepts both provider roots (https://api.example.com) and OpenAI-style
// versioned roots (https://api.example.com/v1) without duplicating /v1.
func Endpoint(baseURL, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiPath := "/" + strings.TrimLeft(path, "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(apiPath, "/v1/") {
		apiPath = strings.TrimPrefix(apiPath, "/v1")
	}
	return base + apiPath
}
