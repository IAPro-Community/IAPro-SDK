// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package spec

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/runtime"
)

var agySpec = Spec{
	ClearLine:        "\x15",
	ID:               "agy",
	DisplayName:      "Antigravity (AGY)",
	ShortBadge:       "AGY",
	InstallGuideURL:  "https://antigravity.google",
	AuthKind:         AuthKindOAuthCLI,
	BinaryCandidates: []string{"agy"},
	EnvKeys: func(profileHome, binDir, profileName string) map[string]string {
		pathBins := []string{binDir}
		env := map[string]string{
			"HOME":                             profileHome,
			"USERPROFILE":                      profileHome,
			"XDG_CONFIG_HOME":                  filepath.Join(profileHome, ".config"),
			"XDG_CACHE_HOME":                   filepath.Join(profileHome, ".cache"),
			"XDG_DATA_HOME":                    filepath.Join(profileHome, ".local", "share"),
			"XDG_STATE_HOME":                   filepath.Join(profileHome, ".local", "state"),
			"AI_PROFILE":                       profileName,
			"AI_PROVIDER":                      "agy",
			"PYTHON_KEYRING_BACKEND":           "keyring.backends.null.Keyring",
			"AI_HOST_DBUS_SESSION_BUS_ADDRESS": os.Getenv("DBUS_SESSION_BUS_ADDRESS"),
			"DBUS_SESSION_BUS_ADDRESS":         "unix:path=/dev/null",
			"PATH":                             runtime.EnhancedPATH(pathBins...),
		}
		if strings.TrimSpace(binDir) != "" {
			// Contract/tests may lack InternalBinDir; never emit a bare relative name.
			env["BROWSER"] = runtime.BrowserHelperPath(binDir)
		}
		return env
	},
	EnvDelete: []string{"GNOME_KEYRING_CONTROL", "GNOME_KEYRING_PID"},
	Kickoff: func(prompt string) []string {
		return []string{"-p", prompt}
	},
	Resume: func(sessionID string) []string {
		return []string{"--conversation=" + sessionID}
	},
	AutoApproval:  []string{"--dangerously-skip-permissions", "--sandbox", "--print-timeout", "60m"},
	ReviewMode:    nil,
	Quota:         QuotaLiveCLI,
	KnownVersions: []string{"1.0.0"},
	ModelGroups: []ModelGroupSpec{
		{
			Key:             "gemini",
			DisplayName:     "Gemini Models",
			Tier:            GroupTierPrimary,
			Aliases:         []string{"gemini"},
			RequiredWindows: []string{"5h"},
		},
		{
			Key:             "claude_gpt",
			DisplayName:     "Claude & GPT Models",
			Tier:            GroupTierAuxiliary,
			Aliases:         []string{"claude", "gpt", "opus", "sonnet", "haiku"},
			RequiredWindows: []string{"claude_5h"},
		},
	},
	QuotaPolicy: QuotaPolicy{
		DefaultTTL:            5 * time.Minute,
		ProbeTimeout:          8 * time.Second,
		Requires5hForBalanced: true,
	},
	HistorySyncPolicy: HistorySyncPolicy{
		Mode:       HistorySyncLink,
		RootSubdir: ".gemini",
		SharedPaths: []string{
			"antigravity-cli/brain",
			"antigravity-cli/conversations",
			"antigravity-cli/conversation_summaries.db",
			"antigravity-cli/history.jsonl",
			"antigravity-cli/knowledge",
			"antigravity-cli/installed_extensions.json",
			"antigravity-cli/customizations",
		},
		CredentialPaths: []string{
			"antigravity-cli/antigravity-oauth-token",
			"antigravity-cli/antigravity-oauth-token.bak",
			"antigravity-cli/installation_id",
			"antigravity-cli/settings.json",
		},
	},
}
