// SPDX-License-Identifier: MIT

package cost

import (
	"math"
	"strings"
	"testing"
)

func TestTariffParsingAndIdentityBinding(t *testing.T) {
	hash := strings.Repeat("a", 64)
	raw := `{"version":"fixture","currency":"USD","models":{"codex/work/m":{"input_per_million":0,"output_per_million":0,"account_identity_sha256":"` + hash + `"}}}`
	table, err := ParseTariffTable([]byte(raw))
	if err != nil || !table.ConfirmedZero("codex/work", "m", hash) || table.ConfirmedZero("codex/work", "m", strings.Repeat("b", 64)) ||
		table.ConfirmedZero("codex/work", "other", hash) {
		t.Fatalf("incorrect zero tariff binding: %v", err)
	}
	for _, raw := range []string{
		`{"version":"v","Version":"v2","currency":"USD","models":{}}`,
		`{"version":"v","currency":"USD","models":{"p/m":{"input_per_million":0,"output_per_million":0},"p/m":{"input_per_million":1,"output_per_million":1}}}`,
		`{"version":"v","currency":"USD","models":{"p/m":{"input_per_million":0,"INPUT_PER_MILLION":1,"output_per_million":0}}}`,
		`{"version":"v","currency":"USD","models":{"p/m":{"input_per_million":-1,"output_per_million":0}}}`,
		`{"version":"v","currency":"USD","models":{"p/m":{"input_per_million":0}}}`,
		`{"version":"v","currency":"USD","models":{}} {}`,
	} {
		if _, err := ParseTariffTable([]byte(raw)); err == nil {
			t.Fatal("invalid tariff accepted")
		}
	}
	if _, err := LoadTariffFile(t.TempDir()); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestTariffArithmetic(t *testing.T) {
	price := Tariff{InputPerMillion: 1, OutputPerMillion: 1}
	if got, err := price.MicroUSD(1, 1); err != nil || got != 1 {
		t.Fatalf("rounding: %d %v", got, err)
	}
	if _, err := price.MicroUSD(-1, 0); err == nil {
		t.Fatal("negative usage accepted")
	}
	price.InputPerMillion = math.MaxInt64
	if _, err := price.MicroUSD(math.MaxInt64, 0); err == nil {
		t.Fatal("overflow accepted")
	}
}
