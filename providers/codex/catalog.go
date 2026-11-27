// SPDX-License-Identifier: MIT

package codex

import (
	"context"
	"os/exec"

	"github.com/IAPro-Community/IAPro-SDK/model"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/codex"
	"github.com/IAPro-Community/IAPro-SDK/providers/process"
)

// CatalogSource selects where the CLI reads the model catalog from.
type CatalogSource string

const (
	// CatalogBundled reads the offline catalog shipped with the CLI binary.
	CatalogBundled CatalogSource = "bundled"
	// CatalogRemote asks the provider for the account's current catalog.
	CatalogRemote CatalogSource = "remote"
)

// CatalogArgs returns the CLI arguments for the requested source. The host
// chooses the source; the SDK never goes online on its own.
func CatalogArgs(source CatalogSource) []string {
	if source == CatalogRemote {
		return []string{"debug", "models"}
	}
	return []string{"debug", "models", "--bundled"}
}

// LaunchCatalog builds the command for a catalog probe. The host supplies the
// verified binary and its isolated account environment.
func LaunchCatalog(binary string, env []string, dir string, source CatalogSource) process.Launch {
	return func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, binary, CatalogArgs(source)...)
		cmd.Env = env
		if dir != "" {
			cmd.Dir = dir
		}
		return cmd, nil
	}
}

// DecodeCatalog turns captured stdout into a typed catalog.
func DecodeCatalog(raw []byte) ([]parser.Model, error) {
	return parser.ParseCatalog(raw)
}

// UnsupportedModelFailure is the classified result for a model outside the
// account catalog: the host must offer another model instead of retrying.
func UnsupportedModelFailure() model.Failure {
	return parser.CatalogFailure()
}
