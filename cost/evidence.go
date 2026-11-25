// Package cost provides provider-neutral accounting projections. Actual
// reservations, invoices and durable ledgers remain host-owned.
package cost

import (
	"errors"
	"math"
)

type Evidence string

const (
	Empty           Evidence = "EMPTY"
	AttributionOnly Evidence = "ATTRIBUTION_ONLY"
	Partial         Evidence = "PARTIAL"
	Measured        Evidence = "MEASURED"
	Estimated       Evidence = "ESTIMATED"
)

type Observation struct {
	ID        string
	MicroUSD  *int64
	Estimated bool
}

type Summary struct {
	KnownMicroUSD     int64    `json:"known_microusd"`
	EstimatedMicroUSD int64    `json:"estimated_microusd"`
	UnknownCount      int      `json:"unknown_count"`
	Count             int      `json:"count"`
	Evidence          Evidence `json:"evidence"`
}

// Summarize rejects duplicate IDs and overflow instead of manufacturing a
// reliable zero. Measured and estimated spend remain separate.
func Summarize(observations []Observation) (Summary, error) {
	out := Summary{Evidence: Empty}
	ids := make(map[string]bool, len(observations))
	estimatedCount := 0
	for _, o := range observations {
		if o.ID == "" || ids[o.ID] {
			return Summary{}, errors.New("missing or duplicate cost observation ID")
		}
		ids[o.ID] = true
		out.Count++
		if o.MicroUSD == nil {
			out.UnknownCount++
			continue
		}
		if *o.MicroUSD < 0 {
			return Summary{}, errors.New("negative cost observation")
		}
		target := &out.KnownMicroUSD
		if o.Estimated {
			target = &out.EstimatedMicroUSD
			estimatedCount++
		}
		if *o.MicroUSD > math.MaxInt64-*target {
			return Summary{}, errors.New("cost overflow")
		}
		*target += *o.MicroUSD
	}
	switch {
	case out.Count == 0:
	case out.UnknownCount == out.Count:
		out.Evidence = AttributionOnly
	case out.UnknownCount > 0:
		out.Evidence = Partial
	case estimatedCount > 0:
		out.Evidence = Estimated
	default:
		out.Evidence = Measured
	}
	return out, nil
}

// CanReserve is a conservative advisory check, not an atomic reservation.
// Unknown/estimated spend cannot prove headroom for automatic paid fallback.
func CanReserve(s Summary, limit, amount int64) bool {
	return s.Evidence == Measured && s.UnknownCount == 0 && s.EstimatedMicroUSD == 0 &&
		s.KnownMicroUSD >= 0 && limit > 0 && amount > 0 && s.KnownMicroUSD <= limit && amount <= limit-s.KnownMicroUSD
}
