// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package spec

import (
	"path/filepath"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/runtime"
)

var geminiSpec = Spec{
	ClearLine:        "\x15",
	ID:               "gemini",
	DisplayName:      "Gemini CLI",
	ShortBadge:       "GEMINI",
	InstallGuideURL:  "https://github.com/google-gemini/gemini-cli",
	AuthKind:         AuthKindOAuthCLI,
	BinaryCandidates: []string{"gemini"},
	EnvKeys: func(profileHome, binDir, profileName string) map[string]string {
		return map[string]string{
			"HOME":            profileHome,
			"USERPROFILE":     profileHome,
			"GEMINI_CLI_HOME": profileHome,
			"XDG_CONFIG_HOME": filepath.Join(profileHome, ".config"),
			"AI_PROFILE":      profileName,
			"AI_PROVIDER":     "gemini",
			"PATH":            runtime.EnhancedPATH(binDir),
		}
	},
	Kickoff: func(prompt string) []string {
		return []string{"-p", prompt}
	},
	Resume: func(sessionID string) []string {
		return []string{"-r", sessionID}
	},
	AutoApproval:        []string{"--approval-mode=yolo", "--sandbox", "--output-format", "text"},
	ReviewMode:          []string{"--approval-mode=plan", "--sandbox", "--output-format", "text"},
	AutonomousIsolation: IsolationUnverified,
	ReviewIsolation:     IsolationUnverified,
	Quota:               QuotaLiveHTTP,
	KnownVersions:       []string{"0.28.2"},
	ModelGroups: []ModelGroupSpec{
		{
			Key:         "",
			DisplayName: "Gemini",
			Tier:        GroupTierPrimary,
			Aliases:     []string{"gemini"},
		},
	},
	QuotaPolicy: QuotaPolicy{
		DefaultTTL:   5 * time.Minute,
		ProbeTimeout: 8 * time.Second,
	},
	HistorySyncPolicy: HistorySyncPolicy{
		Mode:            HistorySyncLink,
		RootSubdir:      ".gemini",
		SharedPaths:     []string{"history"},
		CredentialPaths: []string{"oauth_creds.json", "google_accounts.json"},
	},
}
