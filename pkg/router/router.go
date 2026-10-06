package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/store"
)

var (
	ErrDailyLimitExceeded   = errors.New("global daily run limit exceeded")
	ErrNoAvailableProviders = errors.New("no available model providers within quota")
)

// SkipReason records why a provider candidate was skipped
type SkipReason struct {
	ProviderID string
	Reason     string
}

// Router manages model selection, quota checks, and fallback policies
type Router struct {
	globalCfg *config.GlobalConfig
	store     *store.Store
}

// NewRouter creates a new Router instance
func NewRouter(cfg *config.GlobalConfig, s *store.Store) *Router {
	return &Router{
		globalCfg: cfg,
		store:     s,
	}
}

// CheckGlobalBudget verifies if system has room for more runs today
func (r *Router) CheckGlobalBudget() (bool, error) {
	today := time.Now().Format("2006-01-02")
	total, err := r.store.GetTotalDailyUsage(today)
	if err != nil {
		return false, err
	}

	maxRuns := r.globalCfg.Limits.MaxRunsPerDay
	if maxRuns > 0 && total >= maxRuns {
		return false, fmt.Errorf("%w: %d/%d used today (%s)",
			ErrDailyLimitExceeded, total, maxRuns, today)
	}

	return true, nil
}

// ResolveCandidate iterates through routing list and finds the first viable provider
func (r *Router) ResolveCandidate(ctx context.Context) (*config.ProviderConfig, []SkipReason, error) {
	cands, skips, err := r.ResolveAllCandidates(ctx)
	if err != nil {
		return nil, skips, err
	}
	return cands[0], skips, nil
}

// ResolveAllCandidates iterates through routing list and returns all viable providers in priority order
func (r *Router) ResolveAllCandidates(ctx context.Context) ([]*config.ProviderConfig, []SkipReason, error) {
	if ok, err := r.CheckGlobalBudget(); !ok {
		return nil, nil, err
	}

	today := time.Now().Format("2006-01-02")
	var skips []SkipReason
	var candidates []*config.ProviderConfig

	for _, modelID := range r.globalCfg.Routing.Models {
		prov, exists := r.globalCfg.Providers[modelID]
		if !exists {
			skips = append(skips, SkipReason{
				ProviderID: modelID,
				Reason:     "provider not found in configuration",
			})
			continue
		}

		if !prov.Enabled {
			skips = append(skips, SkipReason{
				ProviderID: modelID,
				Reason:     "provider disabled",
			})
			continue
		}

		// Check API key requirement unless local (e.g. ollama)
		if prov.Provider != "ollama" {
			key := prov.ResolveAPIKey()
			if key == "" {
				raw := strings.TrimSpace(prov.APIKeyEnv)
				reason := "API key is not configured (paste API key or set environment variable)"
				if raw != "" && config.IsEnvVarName(raw) {
					reason = fmt.Sprintf("Environment variable '%s' is not set", raw)
				}
				skips = append(skips, SkipReason{
					ProviderID: modelID,
					Reason:     reason,
				})
				continue
			}
		}

		// Check provider daily quota limit
		if prov.MaxDailyRuns > 0 {
			used, err := r.store.GetProviderDailyUsage(prov.ID, today)
			if err != nil {
				return nil, skips, fmt.Errorf("failed to check provider usage: %w", err)
			}
			if used >= prov.MaxDailyRuns {
				skips = append(skips, SkipReason{
					ProviderID: modelID,
					Reason:     fmt.Sprintf("daily quota exhausted (%d/%d used)", used, prov.MaxDailyRuns),
				})
				continue
			}
		}

		// Passed all guards
		selected := prov
		candidates = append(candidates, &selected)
	}

	if len(candidates) == 0 {
		var sb strings.Builder
		sb.WriteString("all configured providers skipped:\n")
		for _, s := range skips {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", s.ProviderID, s.Reason))
		}
		return nil, skips, fmt.Errorf("%w:\n%s", ErrNoAvailableProviders, sb.String())
	}

	return candidates, skips, nil
}

// IsRateLimitOrQuotaError checks if an error output suggests hitting provider rate limit or quota
func IsRateLimitOrQuotaError(msg string) bool {
	lower := strings.ToLower(msg)
	patterns := []string{
		"429",
		"rate limit",
		"rate_limit",
		"quota exceeded",
		"resource exhausted",
		"too many requests",
		"insufficient_quota",
		"balance exhausted",
		"credit limit",
		"tokens per minute",
		"tpm",
		"token limit",
		"invalid model reference",
		"server error",
		"503",
		"502",
		"500",
		"overloaded",
	}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
