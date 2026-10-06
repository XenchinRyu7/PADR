package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/padr-runner/padr/pkg/config"
)

// PingResult contains connectivity test results and detected models for an AI provider
type PingResult struct {
	Success         bool
	Latency         time.Duration
	Message         string
	AvailableModels []string
}

// PingProvider tests connectivity and credentials to a provider or custom endpoint
func PingProvider(ctx context.Context, p config.ProviderConfig) PingResult {
	start := time.Now()
	client := &http.Client{Timeout: 6 * time.Second}

	apiKey := p.ResolveAPIKey()

	endpoint := strings.TrimSpace(p.Endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
	endpoint = strings.TrimSuffix(endpoint, "/completions")
	reqURL := ""
	reqHeader := make(http.Header)

	rawKey := strings.TrimSpace(p.APIKeyEnv)
	if p.APIKey != "" {
		rawKey = strings.TrimSpace(p.APIKey)
	}

	missingKeyMsg := "API Key is required (paste key or enter environment variable name)"
	if rawKey != "" && config.IsEnvVarName(rawKey) {
		missingKeyMsg = fmt.Sprintf("Environment variable '%s' is not set in system", rawKey)
	}

	switch strings.ToLower(p.Provider) {
	case "groq":
		reqURL = "https://api.groq.com/openai/v1/models"
		if apiKey == "" {
			return PingResult{Success: false, Message: missingKeyMsg}
		}
		reqHeader.Set("Authorization", "Bearer "+apiKey)

	case "google", "gemini":
		if apiKey == "" {
			return PingResult{Success: false, Message: missingKeyMsg}
		}
		reqURL = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)

	case "openrouter":
		reqURL = "https://openrouter.ai/api/v1/models"
		if apiKey == "" {
			return PingResult{Success: false, Message: missingKeyMsg}
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
		// Probe endpoint for models
		if strings.HasSuffix(endpoint, "/v1") || strings.HasSuffix(endpoint, "/v4") || strings.Contains(endpoint, "/api/") {
			reqURL = endpoint + "/models"
		} else {
			reqURL = endpoint + "/v1/models"
		}
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

	// Fallback probe for custom endpoints that might use /models instead of /v1/models or vice-versa
	if err == nil && resp.StatusCode == 404 && (strings.ToLower(p.Provider) == "custom" || p.Provider == "") {
		altURL := ""
		if strings.HasSuffix(reqURL, "/v1/models") {
			altURL = endpoint + "/models"
		} else if strings.HasSuffix(reqURL, "/models") {
			altURL = endpoint + "/v1/models"
		}
		if altURL != "" {
			if altReq, altErr := http.NewRequestWithContext(ctx, "GET", altURL, nil); altErr == nil {
				for k, vals := range reqHeader {
					for _, v := range vals {
						altReq.Header.Add(k, v)
					}
				}
				if altResp, altDoErr := client.Do(altReq); altDoErr == nil && altResp.StatusCode < 400 {
					_ = resp.Body.Close()
					resp = altResp
				}
			}
		}
	}

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
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		models := parseModelsFromResponse(bodyBytes)
		if len(models) == 0 {
			models = FetchOpenCodeModels(ctx, p.Provider)
		}

		msg := fmt.Sprintf("Online (HTTP %d, %dms)", resp.StatusCode, latency.Milliseconds())
		if len(models) > 0 {
			msg = fmt.Sprintf("Online (%dms) - %d models available", latency.Milliseconds(), len(models))
		}

		return PingResult{
			Success:         true,
			Latency:         latency,
			Message:         msg,
			AvailableModels: models,
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

// parseModelsFromResponse extracts model IDs from OpenAI, Groq, OpenRouter, Google, or Ollama JSON responses
func parseModelsFromResponse(body []byte) []string {
	if len(body) == 0 {
		return nil
	}

	var generic struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt string `json:"prompt"`
			} `json:"pricing"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &generic); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var freeModels []string
	var otherModels []string

	// OpenAI / Groq / OpenRouter data array
	for _, item := range generic.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" && !seen[id] {
			seen[id] = true
			if strings.HasSuffix(id, ":free") || strings.Contains(strings.ToLower(id), "free") || item.Pricing.Prompt == "0" {
				freeModels = append(freeModels, id)
			} else {
				otherModels = append(otherModels, id)
			}
		}
	}

	// Google Gemini or Ollama models array
	for _, item := range generic.Models {
		name := item.Name
		if name == "" {
			name = item.ID
		}
		name = strings.TrimPrefix(name, "models/")
		if name != "" && !seen[name] {
			seen[name] = true
			lower := strings.ToLower(name)
			// Filter out non-chat models (embeddings, tts, image/video generators)
			if strings.Contains(lower, "embedding") ||
				strings.Contains(lower, "tts") ||
				strings.Contains(lower, "veo") ||
				strings.Contains(lower, "lyria") ||
				strings.Contains(lower, "image") {
				continue
			}
			if strings.Contains(lower, "free") {
				freeModels = append(freeModels, name)
			} else {
				otherModels = append(otherModels, name)
			}
		}
	}

	return append(freeModels, otherModels...)
}

// FetchOpenCodeModels queries the local opencode CLI for models matching a provider
func FetchOpenCodeModels(ctx context.Context, providerID string) []string {
	exePath, err := exec.LookPath("opencode")
	if err != nil {
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cmdCtx, exePath, "models").Output()
	if err != nil {
		return nil
	}

	pLower := strings.ToLower(strings.TrimSpace(providerID))
	if pLower == "gemini" {
		pLower = "google"
	}

	prefix := pLower + "/"
	lines := strings.Split(string(out), "\n")
	var models []string
	seen := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		tLower := strings.ToLower(trimmed)
		if strings.HasPrefix(tLower, prefix) {
			m := strings.TrimPrefix(trimmed, line[:len(prefix)])
			if m != "" && !seen[m] {
				seen[m] = true
				mLower := strings.ToLower(m)
				// Filter out non-chat utilities
				if strings.Contains(mLower, "embedding") ||
					strings.Contains(mLower, "tts") ||
					strings.Contains(mLower, "veo") ||
					strings.Contains(mLower, "lyria") ||
					strings.Contains(mLower, "image") {
					continue
				}
				models = append(models, m)
			}
		}
	}

	return models
}
