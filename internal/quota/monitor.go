package quota

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
)

// Monitor handles rate limit and quota monitoring
type Monitor struct {
	store  *state.Store
	logger *slog.Logger
}

// NewMonitor creates a new quota monitor
func NewMonitor(store *state.Store, logger ...*slog.Logger) *Monitor {
	log := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	}
	return &Monitor{
		store:  store,
		logger: log,
	}
}

// ProviderStatus represents the status of a provider's rate limits and quotas
type ProviderStatus struct {
	Provider         string
	RateLimitInfo    *state.RateLimitInfo
	QuotaInfo        *state.QuotaInfo
	RateLimitWarning *Warning
	QuotaWarning     *Warning
	IsHealthy        bool
	ShouldDelay      bool
	RecommendedDelay time.Duration
	LastChecked      time.Time
}

// Warning represents a warning about approaching limits
type Warning struct {
	Level       WarningLevel
	Message     string
	Percentage  float64 // Percentage of limit used
	TimeToReset time.Duration
}

// WarningLevel indicates severity of warning
type WarningLevel string

const (
	WarningNone     WarningLevel = "none"
	WarningInfo     WarningLevel = "info"     // < 70% used
	WarningCaution  WarningLevel = "caution"  // 70-85% used
	WarningWarning  WarningLevel = "warning"  // 85-95% used
	WarningCritical WarningLevel = "critical" // > 95% used
	WarningExceeded WarningLevel = "exceeded" // 100% used
)

// CheckProvider checks rate limits and quotas for a provider
func (m *Monitor) CheckProvider(providerName string, prov provider.Provider) (*ProviderStatus, error) {
	status := &ProviderStatus{
		Provider:    providerName,
		IsHealthy:   true,
		ShouldDelay: false,
		LastChecked: time.Now(),
	}

	// Get rate limit info
	rateLimitInfo, err := prov.GetRateLimitInfo()
	if err == nil && rateLimitInfo != nil {
		// Convert provider.RateLimitInfo to state.RateLimitInfo
		remaining := rateLimitInfo.RequestsRemaining
		limit := rateLimitInfo.RequestsLimit
		resetAt := rateLimitInfo.ResetAt

		stateRateLimitInfo := &state.RateLimitInfo{
			Provider:          providerName,
			RequestsRemaining: &remaining,
			RequestsLimit:     &limit,
			ResetAt:           &resetAt,
			CheckedAt:         time.Now(),
		}

		// Save to store
		if err := m.store.SaveRateLimit(providerName, stateRateLimitInfo); err != nil {
			// Non-fatal - continue
			m.logger.Warn("failed to save rate limit info", "provider", providerName, "error", err)
		}

		status.RateLimitInfo = stateRateLimitInfo

		// Check for warnings
		warning := m.checkRateLimitWarning(stateRateLimitInfo)
		if warning != nil {
			status.RateLimitWarning = warning

			// Determine if we should delay
			if warning.Level == WarningExceeded || warning.Level == WarningCritical {
				status.ShouldDelay = true
				status.RecommendedDelay = time.Until(rateLimitInfo.ResetAt)
				status.IsHealthy = false
			}
		}
	}

	// Get quota info
	quotaInfo, err := prov.GetQuotaInfo()
	if err == nil && quotaInfo != nil {
		// Convert provider.QuotaInfo to state.QuotaInfo
		tokensRemaining := quotaInfo.TokensRemaining
		tokensLimit := quotaInfo.TokensLimit
		costRemaining := quotaInfo.CostRemaining
		costLimit := quotaInfo.CostLimit

		stateQuotaInfo := &state.QuotaInfo{
			Provider:        providerName,
			TokensRemaining: &tokensRemaining,
			TokensLimit:     &tokensLimit,
			CostRemaining:   &costRemaining,
			CostLimit:       &costLimit,
			ResetAt:         quotaInfo.ResetAt,
			CheckedAt:       time.Now(),
		}

		// Save to store
		if err := m.store.SaveQuota(providerName, stateQuotaInfo); err != nil {
			// Non-fatal - continue
			m.logger.Warn("failed to save quota info", "provider", providerName, "error", err)
		}

		status.QuotaInfo = stateQuotaInfo

		// Check for warnings
		warning := m.checkQuotaWarning(stateQuotaInfo)
		if warning != nil {
			status.QuotaWarning = warning

			if warning.Level == WarningExceeded || warning.Level == WarningCritical {
				status.IsHealthy = false
			}
		}
	}

	return status, nil
}

// checkRateLimitWarning checks if rate limit is approaching or exceeded
func (m *Monitor) checkRateLimitWarning(info *state.RateLimitInfo) *Warning {
	if info.RequestsLimit == nil || *info.RequestsLimit == 0 {
		return nil
	}

	requestsRemaining := info.RequestsRemaining
	requestsLimit := info.RequestsLimit
	resetAt := info.ResetAt
	timeToReset := time.Duration(0)

	used := *requestsLimit - *requestsRemaining
	percentage := float64(used) / float64(*requestsLimit) * 100

	var level WarningLevel
	var message string

	if requestsRemaining == nil || *requestsRemaining <= 0 {
		level = WarningExceeded
		if resetAt != nil {
			timeToReset = time.Until(*resetAt)
			message = fmt.Sprintf("Rate limit exceeded! Resets in %s", formatDuration(timeToReset))
		} else {
			message = "Rate limit exceeded"
		}
	} else if percentage >= 95 {
		level = WarningCritical
		message = fmt.Sprintf("Critical: Only %d requests remaining (%.1f%% used)", *requestsRemaining, percentage)
	} else if percentage >= 85 {
		level = WarningWarning
		message = fmt.Sprintf("Warning: %d requests remaining (%.1f%% used)", *requestsRemaining, percentage)
	} else if percentage >= 70 {
		level = WarningCaution
		message = fmt.Sprintf("Caution: %d requests remaining (%.1f%% used)", *requestsRemaining, percentage)
	} else {
		level = WarningInfo
		if resetAt != nil {
			timeToReset = time.Until(*resetAt)
		}
		message = fmt.Sprintf("%d/%d requests remaining (%.1f%% available)",
			*requestsRemaining, *requestsLimit, 100-percentage)
	}

	return &Warning{
		Level:       level,
		Message:     message,
		Percentage:  percentage,
		TimeToReset: timeToReset,
	}
}

// checkQuotaWarning checks if quota is approaching or exceeded
func (m *Monitor) checkQuotaWarning(info *state.QuotaInfo) *Warning {
	var percentage float64
	var message string
	var level WarningLevel

	// Check tokens quota first
	if info.TokensLimit != nil && *info.TokensLimit > 0 {
		remaining := 0
		if info.TokensRemaining != nil {
			remaining = *info.TokensRemaining
		}

		used := *info.TokensLimit - remaining
		percentage = float64(used) / float64(*info.TokensLimit) * 100

		if remaining <= 0 {
			level = WarningExceeded
			message = "Token quota exceeded!"
		} else if percentage >= 95 {
			level = WarningCritical
			message = fmt.Sprintf("Critical: Only %d tokens remaining (%.1f%% used)", remaining, percentage)
		} else if percentage >= 85 {
			level = WarningWarning
			message = fmt.Sprintf("Warning: %d tokens remaining (%.1f%% used)", remaining, percentage)
		} else if percentage >= 70 {
			level = WarningCaution
			message = fmt.Sprintf("Caution: %d tokens remaining (%.1f%% used)", remaining, percentage)
		} else {
			level = WarningInfo
			message = fmt.Sprintf("%d/%d tokens remaining (%.1f%% available)",
				remaining, *info.TokensLimit, 100-percentage)
		}
	}

	// Check cost quota if available
	if info.CostLimit != nil && *info.CostLimit > 0 {
		remaining := 0.0
		if info.CostRemaining != nil {
			remaining = *info.CostRemaining
		}

		used := *info.CostLimit - remaining
		costPercentage := (used / *info.CostLimit) * 100

		// Cost quota is more critical, override if worse
		var costLevel WarningLevel
		var costMessage string

		if remaining <= 0 {
			costLevel = WarningExceeded
			costMessage = "Cost quota exceeded!"
		} else if costPercentage >= 95 {
			costLevel = WarningCritical
			costMessage = fmt.Sprintf("Critical: Only $%.2f remaining (%.1f%% used)", remaining, costPercentage)
		} else if costPercentage >= 85 {
			costLevel = WarningWarning
			costMessage = fmt.Sprintf("Warning: $%.2f remaining (%.1f%% used)", remaining, costPercentage)
		} else if costPercentage >= 70 {
			costLevel = WarningCaution
			costMessage = fmt.Sprintf("Caution: $%.2f remaining (%.1f%% used)", remaining, costPercentage)
		} else {
			costLevel = WarningInfo
			costMessage = fmt.Sprintf("$%.2f/$%.2f remaining (%.1f%% available)",
				remaining, *info.CostLimit, 100-costPercentage)
		}

		// Use worse warning level
		if warningLevelRank(costLevel) > warningLevelRank(level) {
			level = costLevel
			message = costMessage
			percentage = costPercentage
		}
	}

	if message == "" {
		return nil
	}

	return &Warning{
		Level:       level,
		Message:     message,
		Percentage:  percentage,
		TimeToReset: time.Until(info.ResetAt),
	}
}

func warningLevelRank(l WarningLevel) int {
	switch l {
	case WarningExceeded:
		return 5
	case WarningCritical:
		return 4
	case WarningWarning:
		return 3
	case WarningCaution:
		return 2
	case WarningInfo:
		return 1
	default:
		return 0
	}
}

// GetCachedStatus retrieves cached rate limit and quota info from the store
func (m *Monitor) GetCachedStatus(providerName string) (*ProviderStatus, error) {
	status := &ProviderStatus{
		Provider:  providerName,
		IsHealthy: true,
	}

	// Get cached rate limit info
	rateLimitInfo, err := m.store.GetRateLimit(providerName)
	if err == nil && rateLimitInfo != nil {
		status.RateLimitInfo = rateLimitInfo
		status.LastChecked = rateLimitInfo.CheckedAt

		// Check if data is stale (older than 1 minute)
		if time.Since(rateLimitInfo.CheckedAt) > time.Minute {
			// Data is stale, but still show it
			status.RateLimitWarning = &Warning{
				Level:   WarningInfo,
				Message: fmt.Sprintf("Data is stale (last checked %s ago)", formatDuration(time.Since(rateLimitInfo.CheckedAt))),
			}
		} else {
			warning := m.checkRateLimitWarning(rateLimitInfo)
			if warning != nil {
				status.RateLimitWarning = warning

				// Determine if we should delay
				if warning.Level == WarningExceeded || warning.Level == WarningCritical {
					status.ShouldDelay = true
					if rateLimitInfo.ResetAt != nil {
						status.RecommendedDelay = time.Until(*rateLimitInfo.ResetAt)
					}
					status.IsHealthy = false
				}
			}
		}
	}

	// Get cached quota info
	quotaInfo, err := m.store.GetQuota(providerName)
	if err == nil && quotaInfo != nil {
		status.QuotaInfo = quotaInfo

		if time.Since(quotaInfo.CheckedAt) > time.Minute {
			// Data is stale
			if status.QuotaWarning == nil {
				status.QuotaWarning = &Warning{
					Level:   WarningInfo,
					Message: fmt.Sprintf("Data is stale (last checked %s ago)", formatDuration(time.Since(quotaInfo.CheckedAt))),
				}
			}
		} else {
			warning := m.checkQuotaWarning(quotaInfo)
			if warning != nil {
				status.QuotaWarning = warning
			}
		}
	}

	return status, nil
}

// ShouldDelayRequest checks if a request should be delayed due to rate limits
func (m *Monitor) ShouldDelayRequest(providerName string) (bool, time.Duration, error) {
	status, err := m.GetCachedStatus(providerName)
	if err != nil {
		return false, 0, err
	}

	return status.ShouldDelay, status.RecommendedDelay, nil
}

// RefreshProviderStatus forces a refresh of provider status
func (m *Monitor) RefreshProviderStatus(providerName string, prov provider.Provider) (*ProviderStatus, error) {
	return m.CheckProvider(providerName, prov)
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// GetWarningSymbol returns an emoji symbol for the warning level
func GetWarningSymbol(level WarningLevel) string {
	switch level {
	case WarningExceeded:
		return "🚫"
	case WarningCritical:
		return "🔴"
	case WarningWarning:
		return "⚠️ "
	case WarningCaution:
		return "🟡"
	case WarningInfo:
		return "ℹ️ "
	default:
		return "✅"
	}
}
