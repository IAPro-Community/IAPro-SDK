// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

func LookPath(name string) (string, error) {
	if resolved, err := ResolveCommand(name); err == nil {
		return resolved.ArtifactPath, nil
	}
	// 1. Standard PATH
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// 2. Proactive developer directories
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		candidates := []string{
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, ".bun", "bin", name),
			filepath.Join(home, ".opencode", "bin", name),
			filepath.Join(home, ".cargo", "bin", name),
			filepath.Join(home, ".local", "share", "pnpm", name),
			filepath.Join("/usr", "local", "bin", name),
			filepath.Join("/snap", "bin", name),
		}

		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
				return cand, nil
			}
		}

		// Check NVM directories
		nvmPattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", name)
		if matches, err := filepath.Glob(nvmPattern); err == nil && len(matches) > 0 {
			for i := len(matches) - 1; i >= 0; i-- {
				cand := matches[i]
				if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
					return cand, nil
				}
			}
		}
	}

	return "", exec.ErrNotFound
}

// EnhancedPATH builds a robust, prioritized and deduplicated PATH string
// including the provider binary directory, active developer toolchains (Node/NVM, Bun, Cargo, PNPM),
// user directories, and standard system paths.
func EnhancedPATH(extraDirs ...string) string {
	home, _ := os.UserHomeDir()
	var candidates []string
	candidates = append(candidates, extraDirs...)

	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, ".local", "share", "pnpm"),
		)

		// Include all installed NVM node versions (most recent first)
		nvmPattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin")
		if matches, err := filepath.Glob(nvmPattern); err == nil && len(matches) > 0 {
			for i := len(matches) - 1; i >= 0; i-- {
				candidates = append(candidates, matches[i])
			}
		}

		// Also check fnm and asdf shims
		candidates = append(candidates,
			filepath.Join(home, ".fnm", "current", "bin"),
			filepath.Join(home, ".asdf", "shims"),
		)
	}

	candidates = append(candidates,
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/local/sbin",
		"/usr/sbin",
		"/sbin",
	)

	// Append existing PATH
	existing := os.Getenv("PATH")
	if existing != "" {
		candidates = append(candidates, strings.Split(existing, string(os.PathListSeparator))...)
	}

	// Deduplicate and verify directory exists on disk
	seen := make(map[string]bool)
	var finalDirs []string
	for _, dir := range candidates {
		dir = strings.TrimSpace(dir)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			finalDirs = append(finalDirs, dir)
		}
	}

	return strings.Join(finalDirs, string(os.PathListSeparator))
}

// EnvSet applies environment overrides and removes unset keys from the base environment slice.
func EnvSet(base []string, overrides map[string]string, unset ...string) []string {
	unsetMap := make(map[string]bool)
	for _, k := range unset {
		unsetMap[k] = true
	}

	overrideKeys := make(map[string]bool)
	for k := range overrides {
		overrideKeys[k] = true
	}

	var out []string
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		k := parts[0]
		if unsetMap[k] || overrideKeys[k] {
			continue
		}
		out = append(out, entry)
	}

	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func BrowserHelperPath(binDir string) string {
	name := "ai-browser"
	if stdruntime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(binDir, name)
}
