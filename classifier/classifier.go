// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE for source and copyright.

package classifier

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

var (
	rateLimitPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(rate[ -]?limit(ed)?|too many requests|http 429|status 429)\b`),
		regexp.MustCompile(`(?i)\b(quota exceeded|usage limit reached|exceeded your current quota)\b`),
		regexp.MustCompile(`(?i)\b(capacity reached|temporarily rate-limited)\b`),
	}

	authPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(auth(entication)? required|unauthorized|http 401|status 401)\b`),
		regexp.MustCompile(`(?i)\b(token expired|session expired|invalid token|invalid api key)\b`),
		regexp.MustCompile(`(?i)\b(please login|run .*login|not logged in)\b`),
		regexp.MustCompile(`(?i)\b(access denied|permission denied|forbidden|http 403)\b`),
	}

	quotaExhaustedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(insufficient credits|out of credits|balance too low|billing limit)\b`),
		regexp.MustCompile(`(?i)\b(credit balance is too low|plan quota exhausted)\b`),
	}

	networkPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(connection refused|network unreachable|dial tcp|no route to host)\b`),
		regexp.MustCompile(`(?i)\b(tls handshake timeout|i/o timeout|connection reset by peer)\b`),
	}

	providerErrorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(internal server error|http 500|http 502|http 503|http 504)\b`),
		regexp.MustCompile(`(?i)\b(service unavailable|bad gateway|gateway timeout|model overloaded)\b`),
	}

	retrySecondsRegex = regexp.MustCompile(`(?i)(?:retry|try again|wait)\s+(?:in|after)?\s*(\d+)\s*(?:seconds?|secs?|s)\b`)
	retryMinutesRegex = regexp.MustCompile(`(?i)(?:retry|try again|wait)\s+(?:in|after)?\s*(\d+)\s*(?:minutes?|mins?|m)\b`)
	retryHoursRegex   = regexp.MustCompile(`(?i)(?:retry|try again|wait)\s+(?:in|after)?\s*(\d+)\s*(?:hours?|hrs?|h)\b`)
)

// Classify inspects an error and output log to determine the failure kind and retry parameters.
func Classify(err error, output string) model.Failure {
	combined := ""
	if err != nil {
		combined += err.Error() + "\n"
	}
	combined += output

	if strings.TrimSpace(combined) == "" {
		return model.Failure{Kind: model.FailureNone}
	}

	// 1. Check Rate Limit
	for _, p := range rateLimitPatterns {
		if p.MatchString(combined) {
			dur := ExtractRetryDuration(combined)
			var resetAt *time.Time
			if dur > 0 {
				t := time.Now().Add(dur)
				resetAt = &t
			}
			return model.Failure{
				Kind:       model.FailureRateLimit,
				Message:    "Rate limit exceeded on provider",
				RetryAfter: &dur,
				ResetAt:    resetAt,
			}
		}
	}

	// 2. Check Quota Exhaustion
	for _, p := range quotaExhaustedPatterns {
		if p.MatchString(combined) {
			return model.Failure{
				Kind:    model.FailureQuota,
				Message: "Account quota / credit limit reached",
			}
		}
	}

	// 3. Check Auth Failure
	for _, p := range authPatterns {
		if p.MatchString(combined) {
			return model.Failure{
				Kind:    model.FailureAuth,
				Message: "Authentication required or credentials expired",
			}
		}
	}

	// 4. Check Network Failure
	for _, p := range networkPatterns {
		if p.MatchString(combined) {
			return model.Failure{
				Kind:    model.FailureNetwork,
				Message: "Network connection failure to AI provider",
			}
		}
	}

	// 5. Check Provider Transient Error
	for _, p := range providerErrorPatterns {
		if p.MatchString(combined) {
			return model.Failure{
				Kind:    model.FailureProvider,
				Message: "AI provider service unavailable or overloaded",
			}
		}
	}

	return model.Failure{
		Kind:    model.FailureUnknown,
		Message: "CLI command returned non-zero exit",
	}
}

// ExtractRetryDuration attempts to parse retry cooldown intervals from error messages.
func ExtractRetryDuration(s string) time.Duration {
	for _, unit := range []struct {
		pattern *regexp.Regexp
		scale   time.Duration
	}{
		{retrySecondsRegex, time.Second},
		{retryMinutesRegex, time.Minute},
		{retryHoursRegex, time.Hour},
	} {
		if m := unit.pattern.FindStringSubmatch(s); len(m) > 1 {
			val, err := strconv.ParseUint(m[1], 10, 64)
			if err != nil || val == 0 || val > uint64(math.MaxInt64/int64(unit.scale)) {
				return 0
			}
			return time.Duration(val) * unit.scale
		}
	}
	return 0
}
