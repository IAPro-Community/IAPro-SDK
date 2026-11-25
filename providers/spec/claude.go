// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package spec

import (
	"path/filepath"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/runtime"
)

var claudeSpec = Spec{
	ClearLine:        "\x15",
	ID:               "claude",
	DisplayName:      "Claude Code",
	ShortBadge:       "CLAUDE",
	InstallGuideURL:  "https://docs.anthropic.com/en/docs/claude-code",
	AuthKind:         AuthKindOAuthCLI,
	BinaryCandidates: []string{"claude"},
	EnvKeys: func(profileHome, binDir, profileName string) map[string]string {
		return map[string]string{
			"HOME":              profileHome,
			"USERPROFILE":       profileHome,
			"CLAUDE_CONFIG_DIR": filepath.Join(profileHome, ".claude"),
			"XDG_CONFIG_HOME":   filepath.Join(profileHome, ".config"),
			"AI_PROFILE":        profileName,
			"AI_PROVIDER":       "claude",
			"PATH":              runtime.EnhancedPATH(binDir),
		}
	},
	Kickoff: func(prompt string) []string {
		return []string{"-p", prompt}
	},
	Resume: func(sessionID string) []string {
		return []string{"--resume", sessionID}
	},
	AutoApproval:  []string{"--dangerously-skip-permissions", "--output-format", "text"},
	ReviewMode:    []string{"--permission-mode", "plan", "--output-format", "text"},
	Quota:         QuotaLiveHTTP,
	KnownVersions: []string{"2.1.250"},
	ModelGroups: []ModelGroupSpec{
		{
			Key:         "",
			DisplayName: "Claude",
			Tier:        GroupTierPrimary,
			Aliases:     []string{"claude", "sonnet", "opus", "haiku"},
		},
	},
	QuotaPolicy: QuotaPolicy{
		DefaultTTL:   5 * time.Minute,
		ProbeTimeout: 8 * time.Second,
	},
	HistorySyncPolicy: HistorySyncPolicy{
		Mode:            HistorySyncLink,
		RootSubdir:      ".claude",
		SharedPaths:     []string{"projects", "history.jsonl", "sessions", "file-history", "plans"},
		CredentialPaths: []string{".credentials.json", "credentials.json", "../.claude.json"},
	},
}
