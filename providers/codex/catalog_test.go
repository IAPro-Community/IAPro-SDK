// SPDX-License-Identifier: MIT

package codex

import (
	"context"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestCatalogArgs(t *testing.T) {
	if got := CatalogArgs(CatalogBundled); len(got) != 3 || got[2] != "--bundled" {
		t.Fatalf("bundled args: %v", got)
	}
	if got := CatalogArgs(CatalogRemote); len(got) != 2 || got[1] != "models" {
		t.Fatalf("remote args: %v", got)
	}
	if got := CatalogArgs(CatalogSource("unknown")); len(got) != 3 {
		t.Fatalf("unknown source must stay offline: %v", got)
	}
}

func TestLaunchCatalogUsesHostBinaryAndEnv(t *testing.T) {
	cmd, err := LaunchCatalog("codex", []string{"HOME=/tmp/home"}, "/tmp/wd", CatalogBundled)(context.Background())
	if err != nil || cmd == nil {
		t.Fatal(err)
	}
	if cmd.Path != "codex" && cmd.Args[0] != "codex" {
		t.Fatalf("unexpected binary: %v", cmd.Args)
	}
	if cmd.Dir != "/tmp/wd" || len(cmd.Env) != 1 || cmd.Env[0] != "HOME=/tmp/home" {
		t.Fatalf("host policy not applied: %+v", cmd)
	}
}

func TestDecodeCatalogRejectsEmpty(t *testing.T) {
	if _, err := DecodeCatalog(nil); err == nil {
		t.Fatal("expected an error")
	}
	if f := UnsupportedModelFailure(); f.Kind != model.FailureModelUnsupported {
		t.Fatalf("unexpected failure: %+v", f)
	}
}
