package router

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/padr-runner/padr/pkg/config"
)

// PingResult contains connectivity test results for an AI provider
type PingResult struct {
	Success bool
	Latency time.Duration
	Message string
}

// PingProvider tests connectivity and credentials to a provider or custom endpoint
func PingProvider(ctx context.Context, p config.ProviderConfig) PingResult {
	start := time.Now()
	client := &http.Client{Timeout: 6 * time.Second}

	apiKey := ""
	if p.APIKeyEnv != "" {
		apiKey = os.Getenv(p.APIKeyEnv)
	}

	endpoint := strings.TrimRight(p.Endpoint, "/")
	reqURL := ""
	reqHeader := make(http.Header)

	switch strings.ToLower(p.Provider) {
	case "groq":
		reqURL = "https://api.groq.com/openai/v1/models"
		if apiKey == "" {
			return PingResult{Success: false, Message: fmt.Sprintf("Environment variable %s is not set", p.APIKeyEnv)}
		}
		reqHeader.Set("Authorization", "Bearer "+apiKey)

	case "google", "gemini":
		if apiKey == "" {
			return PingResult{Success: false, Message: fmt.Sprintf("Environment variable %s is not set", p.APIKeyEnv)}
		}
		reqURL = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)

	case "openrouter":
		reqURL = "https://openrouter.ai/api/v1/models"
		if apiKey == "" {
			return PingResult{Success: false, Message: fmt.Sprintf("Environment variable %s is not set", p.APIKeyEnv)}
		}
		reqHeader.Set("Authorization", "Bearer "+apiKey)

	case "ollama":
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		reqURL = endpoint + "/api/tags"

	default: // custom URL (e.g. z.ai, LM Studio, vLLM, OpenAI-compatible)
		if endpoint == "" {
			return PingResult{Success: false, Message: "Custom provider requires Endpoint URL"}
		}
		if apiKey != "" {
			reqHeader.Set("Authorization", "Bearer "+apiKey)
		}
		// Probe /v1/models or base endpoint
		reqURL = endpoint + "/v1/models"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return PingResult{Success: false, Message: err.Error()}
	}

	for k, vals := range reqHeader {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		// Fallback probe for custom endpoints that might not have /v1/models
		if endpoint != "" {
			fallbackReq, fbErr := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if fbErr == nil {
				if fbResp, fbDoErr := client.Do(fallbackReq); fbDoErr == nil {
					_ = fbResp.Body.Close()
					return PingResult{
						Success: true,
						Latency: time.Since(start),
						Message: fmt.Sprintf("Reachable (HTTP %d, %dms)", fbResp.StatusCode, time.Since(start).Milliseconds()),
					}
				}
			}
		}
		return PingResult{Success: false, Latency: latency, Message: fmt.Sprintf("Connection failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return PingResult{
			Success: true,
			Latency: latency,
			Message: fmt.Sprintf("Online (HTTP %d, %dms)", resp.StatusCode, latency.Milliseconds()),
		}
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return PingResult{
			Success: false,
			Latency: latency,
			Message: fmt.Sprintf("Authentication failed: Invalid API key (HTTP %d)", resp.StatusCode),
		}
	}

	return PingResult{
		Success: false,
		Latency: latency,
		Message: fmt.Sprintf("Server returned HTTP %d", resp.StatusCode),
	}
}
