// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE for source and copyright.

package quota

import (
	"runtime"
	"strings"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// RenderProgressBar builds an honest visual progress bar.
func RenderProgressBar(status model.UsageStatus, remainingPercent *float64, width int) string {
	if width < 6 {
		width = 10
	}

	switch status {
	// RATE_LIMITED now carries exact percentages when it comes from the official
	// provider read. Rendering the bar keeps the consumption visible; the status
	// column is what tells the operator the account is blocked.
	case model.UsageLive, model.UsageCached, model.UsageEstimated, model.UsageRateLimited:
		if remainingPercent == nil {
			if status == model.UsageRateLimited {
				return formatProgressLabel("LIMITED", width)
			}
			return formatProgressLabel("UNKNOWN", width)
		}
		pct := *remainingPercent
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		fillCount := int((pct / 100.0) * float64(width))
		if fillCount > width {
			fillCount = width
		}
		emptyCount := width - fillCount

		// On Windows console (cmd.exe, PowerShell, legacy OEM code pages CP437/CP850/CP1252),
		// UTF-8 block characters (█, ░) frequently render as '???' or mojibake.
		// We use universal ASCII characters (# and -) on Windows to ensure 100% clean rendering.
		if runtime.GOOS == "windows" {
			return "[" + strings.Repeat("#", fillCount) + strings.Repeat("-", emptyCount) + "]"
		}
		return "[" + strings.Repeat("█", fillCount) + strings.Repeat("░", emptyCount) + "]"

	case model.UsageUnsupported:
		return formatProgressLabel("UNSUPPORT", width)

	case model.UsageError:
		return formatProgressLabel("ERROR", width)

	default:
		return formatProgressLabel("UNKNOWN", width)
	}
}

func formatProgressLabel(label string, width int) string {
	if len(label) >= width {
		return "[" + label[:width] + "]"
	}
	pad := width - len(label)
	left := pad / 2
	right := pad - left
	return "[" + strings.Repeat(" ", left) + label + strings.Repeat(" ", right) + "]"
}
