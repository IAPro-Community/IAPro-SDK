// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package spec

import "github.com/IAPro-Community/IAPro-SDK/runtime"

func overlayEnv(base []string, overrides map[string]string, unset []string) []string {
	return runtime.EnvSet(base, overrides, unset...)
}
