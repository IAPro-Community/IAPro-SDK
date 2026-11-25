package coordination

import (
	"errors"
	"testing"
	"time"
)

func TestAdmissionReplayLimitsAndResolution(t *testing.T) {
	now := time.Now()
	in := Admission{OwnerID: "owner", RunID: "run", SourceTaskID: "task", SourceAgent: "agent", TurnID: "turn", Authorized: true, MaxDepth: 2}
	raw := CoordinationRequest{Operation: CoordinationDelegate, Subject: "review", Objective: "check"}
	req, replay, err := Admit(in, raw, now)
	if err != nil || replay || req.Status != CoordinationPending {
		t.Fatalf("admission: %+v %v %v", req, replay, err)
	}
	in.Existing = []CoordinationRequest{req}
	in.TurnCount = MaxRequestsPerTurn
	_, replay, err = Admit(in, raw, now)
	if err != nil || !replay {
		t.Fatal("exact replay should not consume another slot")
	}
	raw.Subject = "different"
	if _, _, err := Admit(in, raw, now); !errors.Is(err, ErrCoordinationLimit) {
		t.Fatal("turn limit ignored")
	}
	in.TurnCount = 0
	in.TargetTaskID = "ancestor"
	in.Ancestors = []string{"ancestor"}
	if _, _, err := Admit(in, raw, now); !errors.Is(err, ErrCoordinationDenied) {
		t.Fatal("cycle accepted")
	}
	resolved, err := Resolve(req, CoordinationResolved, "done", "receipt", true, now)
	if err != nil || resolved.ResolvedAt == nil {
		t.Fatal(err)
	}
	if _, err := Resolve(resolved, CoordinationFailed, "failed", "", true, now); !errors.Is(err, ErrCoordinationConflict) {
		t.Fatal("terminal state overwritten")
	}
}
