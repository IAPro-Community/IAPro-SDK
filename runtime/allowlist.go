// SPDX-License-Identifier: MIT

package runtime

import "strings"

// AllowEnvironment constructs an explicit environment. Names are compared
// case-insensitively so a product can use the same secret boundary on Windows
// and Unix. The host owns the allowlist and all credential injection.
func AllowEnvironment(parent []string, allowed ...string) []string {
	names := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		if name != "" && !strings.ContainsAny(name, "=\x00") {
			names[strings.ToUpper(name)] = struct{}{}
		}
	}
	out := make([]string, 0, len(parent))
	for _, entry := range parent {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || strings.ContainsRune(entry, 0) {
			continue
		}
		if _, ok := names[strings.ToUpper(name)]; ok {
			out = append(out, entry)
		}
	}
	return out
}
