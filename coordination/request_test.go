package coordination

import (
	"strings"
	"testing"
)

func TestRequestContract(t *testing.T) {
	r := CoordinationRequest{RunID: "run", SourcePackageID: "task", SourceAgentID: "agent", Operation: CoordinationDelegate, Subject: "Review", Objective: "Check result", Status: CoordinationPending}
	r.ID = RequestID(r)
	if err := ValidateCoordinationRequest(r); err != nil {
		t.Fatal(err)
	}
	copy := CloneRequest(r)
	if RequestID(copy) != r.ID || !SemanticallyEqual(r, copy) {
		t.Fatal("semantic replay changed")
	}
	copy.Objective = "Different work"
	if RequestID(copy) == r.ID || SemanticallyEqual(r, copy) {
		t.Fatal("different request must not be replayed")
	}
	copy.Objective = ""
	if ValidateCoordinationRequest(copy) == nil {
		t.Fatal("delegation without objective")
	}
	copy.Objective = strings.Repeat("x", 8193)
	if ValidateCoordinationRequest(copy) == nil {
		t.Fatal("oversized request")
	}
	copy = r
	copy.Operation = "EXECUTE"
	if ValidateCoordinationRequest(copy) == nil {
		t.Fatal("unknown operation")
	}
}
