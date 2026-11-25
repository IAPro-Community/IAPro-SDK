// SPDX-License-Identifier: MIT

package cost

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"strings"
)

const MaxTariffBytes = 1 << 20

// Tariff describes USD micros per million tokens, not an invoice or a
// subscription's marginal cost unless the host explicitly verifies it.
type Tariff struct {
	InputPerMillion       int64
	OutputPerMillion      int64
	AccountIdentitySHA256 string
}

type TariffTable struct {
	Version string
	Models  map[string]Tariff
}

func ValidIdentityHash(hash string) bool {
	if len(hash) != 64 || strings.ToLower(hash) != hash {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func (t Tariff) Validate() error {
	if t.InputPerMillion < 0 || t.OutputPerMillion < 0 ||
		(t.AccountIdentitySHA256 != "" && !ValidIdentityHash(t.AccountIdentitySHA256)) {
		return errors.New("invalid tariff entry")
	}
	return nil
}

func (t TariffTable) Validate() error {
	if len(t.Models) > 0 && strings.TrimSpace(t.Version) == "" {
		return errors.New("tariff version required")
	}
	for key, entry := range t.Models {
		if !validTariffKey(key) || entry.Validate() != nil {
			return errors.New("invalid tariff entry")
		}
	}
	return nil
}

func validTariffKey(key string) bool {
	provider, model, ok := strings.Cut(key, "/")
	return ok && provider != "" && model != "" && strings.TrimSpace(key) == key && !strings.ContainsRune(key, 0)
}

func (t TariffTable) Lookup(provider, model string) (Tariff, bool) {
	entry, ok := t.Models[provider+"/"+model]
	return entry, ok
}

// ConfirmedZero requires explicit prices and identity binding. The host must
// independently verify account terms; absence of a tariff never means free.
func (t TariffTable) ConfirmedZero(provider, model, identity string) bool {
	entry, ok := t.Lookup(provider, model)
	return ok && strings.TrimSpace(t.Version) != "" && ValidIdentityHash(identity) &&
		entry.AccountIdentitySHA256 == identity && entry.InputPerMillion == 0 && entry.OutputPerMillion == 0
}

// MicroUSD calculates a non-negative integer charge rounded up. Unknown
// usage must be handled by the caller before invoking this function.
func (t Tariff) MicroUSD(input, output int64) (int64, error) {
	if input < 0 || output < 0 || t.Validate() != nil {
		return 0, errors.New("invalid tariff or token usage")
	}
	var total, component big.Int
	total.Mul(big.NewInt(input), big.NewInt(t.InputPerMillion))
	component.Mul(big.NewInt(output), big.NewInt(t.OutputPerMillion))
	total.Add(&total, &component)
	unit := big.NewInt(1000000)
	var quotient, remainder big.Int
	quotient.QuoRem(&total, unit, &remainder)
	if remainder.Sign() != 0 {
		quotient.Add(&quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, errors.New("tariff calculation overflow")
	}
	return quotient.Int64(), nil
}

// ParseTariffTable strictly parses a versioned USD table. Duplicate object
// keys (including case variants of struct fields) are not accepted.
func ParseTariffTable(data []byte) (TariffTable, error) {
	if len(data) > MaxTariffBytes || len(data) == 0 {
		return TariffTable{}, errors.New("invalid tariff size")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := checkTariffJSON(dec, 0); err != nil {
		return TariffTable{}, errors.New("invalid or duplicate tariff JSON field")
	}
	if _, err := dec.Token(); err != io.EOF {
		return TariffTable{}, errors.New("trailing tariff JSON data")
	}
	var wire struct {
		Version  string `json:"version"`
		Currency string `json:"currency"`
		Models   map[string]struct {
			Input    *int64 `json:"input_per_million"`
			Output   *int64 `json:"output_per_million"`
			Identity string `json:"account_identity_sha256"`
		} `json:"models"`
	}
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil || wire.Currency != "USD" ||
		strings.TrimSpace(wire.Version) == "" || wire.Models == nil {
		return TariffTable{}, errors.New("invalid tariff table")
	}
	table := TariffTable{Version: wire.Version, Models: make(map[string]Tariff, len(wire.Models))}
	for key, entry := range wire.Models {
		if entry.Input == nil || entry.Output == nil {
			return TariffTable{}, errors.New("missing tariff price")
		}
		table.Models[key] = Tariff{InputPerMillion: *entry.Input, OutputPerMillion: *entry.Output, AccountIdentitySHA256: entry.Identity}
	}
	if err := table.Validate(); err != nil {
		return TariffTable{}, err
	}
	return table, nil
}

func checkTariffJSON(dec *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("tariff JSON nesting limit")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := make(map[string]struct{})
		for dec.More() {
			raw, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := raw.(string)
			if !ok {
				return errors.New("invalid JSON key")
			}
			// The models object is a case-sensitive map; wire fields are not.
			if depth != 1 {
				key = strings.ToUpper(key)
			}
			if _, exists := keys[key]; exists {
				return errors.New("duplicate JSON field")
			}
			keys[key] = struct{}{}
			if err := checkTariffJSON(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := checkTariffJSON(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = dec.Token()
	return err
}

// LoadTariffFile reads a bounded regular file from a host-authorized path.
// The host must keep the directory under trusted control; file checks alone
// cannot protect against concurrent malicious path replacement.
func LoadTariffFile(path string) (TariffTable, error) {
	if path == "" {
		return TariffTable{}, nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxTariffBytes {
		return TariffTable{}, errors.New("tariff must be a bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return TariffTable{}, errors.New("cannot open tariff file")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return TariffTable{}, errors.New("tariff file changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxTariffBytes+1))
	if err != nil {
		return TariffTable{}, errors.New("cannot read tariff file")
	}
	return ParseTariffTable(data)
}
