package routing

import (
	"errors"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

type HandoffPhase string

const (
	HandoffRequested    HandoffPhase = "REQUESTED"
	HandoffValidated    HandoffPhase = "TARGET_VALIDATED"
	HandoffCheckpointed HandoffPhase = "CHECKPOINTED"
	HandoffQuiesced     HandoffPhase = "SOURCE_QUIESCED"
	HandoffStarting     HandoffPhase = "TARGET_STARTING"
	HandoffVerified     HandoffPhase = "VERIFIED"
	HandoffCompleted    HandoffPhase = "COMPLETED"
	HandoffRollback     HandoffPhase = "ROLLBACK_REQUIRED"
	HandoffRolledBack   HandoffPhase = "ROLLED_BACK"
	HandoffUnsafe       HandoffPhase = "FAILED_UNSAFE"
)

var ErrHandoffTransition = errors.New("handoff transition refused")

// HandoffRecord is persisted by the host with CAS on Revision under a fenced
// lease. Session IDs are provider references, never copied credential state.
type HandoffRecord struct {
	ID           string
	OwnerID      string
	Source       model.AccountScope
	Target       model.AccountScope
	SessionID    string
	CheckpointID string
	Phase        HandoffPhase
	Revision     uint64
}

type HandoffEvidence struct {
	Authorized        bool
	LeaseValid        bool
	TargetVerified    bool
	ResumeSupported   bool
	CheckpointID      string
	SourceStopped     bool
	TargetStopped     bool
	ProviderConfirmed bool
	SourceRestored    bool
}

// AdvanceHandoff describes safe transitions only. The host persists intent
// before each external action, reconciles on restart and uses idempotency IDs.
// A running process or successful launch alone cannot confirm continuity.
func AdvanceHandoff(r HandoffRecord, next HandoffPhase, e HandoffEvidence) (HandoffRecord, error) {
	if r.ID == "" || r.OwnerID == "" || !r.Source.Verifiable() || !r.Target.Verifiable() ||
		r.Source.ProviderID != r.Target.ProviderID || r.Source.AccountID == r.Target.AccountID || r.Source.ProfileID == r.Target.ProfileID ||
		r.SessionID == "" || !e.Authorized || !e.LeaseValid || r.Revision == ^uint64(0) {
		return HandoffRecord{}, ErrHandoffTransition
	}
	allowed := false
	switch r.Phase {
	case HandoffRequested:
		allowed = next == HandoffValidated && e.TargetVerified && e.ResumeSupported
	case HandoffValidated:
		allowed = next == HandoffCheckpointed && e.CheckpointID != ""
		if allowed {
			r.CheckpointID = e.CheckpointID
		}
	case HandoffCheckpointed:
		allowed = next == HandoffQuiesced && r.CheckpointID != "" && e.SourceStopped
	case HandoffQuiesced:
		allowed = next == HandoffStarting && e.SourceStopped && e.TargetVerified && e.ResumeSupported || next == HandoffRollback
	case HandoffStarting:
		allowed = next == HandoffVerified && e.SourceStopped && e.ProviderConfirmed || next == HandoffRollback || next == HandoffUnsafe
	case HandoffVerified:
		allowed = next == HandoffCompleted && e.SourceStopped && e.ProviderConfirmed || next == HandoffRollback
	case HandoffRollback:
		allowed = next == HandoffRolledBack && e.TargetStopped && e.SourceRestored || next == HandoffUnsafe
	}
	if !allowed {
		return HandoffRecord{}, ErrHandoffTransition
	}
	r.Phase = next
	r.Revision++
	return r, nil
}
