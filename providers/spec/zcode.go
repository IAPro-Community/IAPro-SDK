package spec

import (
	"path/filepath"

	"github.com/IAPro-Community/IAPro-SDK/runtime"
)

// ZCodeSpec describes the CLI contract; availability, installation and
// authorization must be established by the consuming host.
func ZCodeSpec() Spec {
	return Spec{
		ID:               "zcode",
		DisplayName:      "Z Code",
		ShortBadge:       "ZCD",
		AuthKind:         AuthKindAPIKey,
		BinaryCandidates: []string{"zcode"},
		ClearLine:        "\x15",
		EnvKeys: func(home, binDir, profile string) map[string]string {
			return map[string]string{
				"HOME":            home,
				"USERPROFILE":     home,
				"ZCODE_HOME":      home,
				"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
				"AI_PROFILE":      profile,
				"AI_PROVIDER":     "zcode",
				"PATH":            runtime.EnhancedPATH(binDir),
			}
		},
		Kickoff:           func(prompt string) []string { return []string{"-p", prompt} },
		Resume:            func(id string) []string { return []string{"--resume", id} },
		AutoApproval:      []string{"--mode", "yolo"},
		ReviewMode:        []string{"--mode", "guarded"},
		Quota:             QuotaUnsupported,
		KnownVersions:     []string{"3.17.2-44"},
		HistorySyncPolicy: HistorySyncPolicy{Mode: HistorySyncNone},
	}
}
