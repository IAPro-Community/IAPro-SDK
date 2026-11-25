package routing

import (
	"errors"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestHandoffRequiresProofOfContinuity(t *testing.T) {
	r := HandoffRecord{ID: "handoff", OwnerID: "owner", SessionID: "session", Phase: HandoffRequested,
		Source: model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "a", IdentityVersion: "1"},
		Target: model.AccountScope{ProviderID: "codex", ProfileID: "b", AccountID: "b", IdentityVersion: "1"}}
	e := HandoffEvidence{Authorized: true, LeaseValid: true, TargetVerified: true, ResumeSupported: true, CheckpointID: "checkpoint", SourceStopped: true}
	for _, phase := range []HandoffPhase{HandoffValidated, HandoffCheckpointed, HandoffQuiesced, HandoffStarting} {
		var err error
		r, err = AdvanceHandoff(r, phase, e)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := AdvanceHandoff(r, HandoffVerified, e); !errors.Is(err, ErrHandoffTransition) {
		t.Fatal("launch without confirmation accepted")
	}
	e.ProviderConfirmed = true
	r, err := AdvanceHandoff(r, HandoffVerified, e)
	if err != nil {
		t.Fatal(err)
	}
	r, err = AdvanceHandoff(r, HandoffCompleted, e)
	if err != nil || r.Revision != 6 {
		t.Fatalf("unexpected state: %+v %v", r, err)
	}
}
