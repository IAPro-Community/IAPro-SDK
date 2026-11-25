package coordination

import (
	"errors"
	"strings"
	"time"
)

var ErrCoordinationConflict = errors.New("coordination request conflict")
var ErrCoordinationLimit = errors.New("coordination limit reached")
var ErrCoordinationDenied = errors.New("coordination request not authorized")

// Admission is a trusted host snapshot obtained within the same transaction
// that persists the request. It is never derived from model output.
type Admission struct {
	OwnerID      string
	RunID        string
	SourceTaskID string
	SourceAgent  string
	TurnID       string
	Authorized   bool
	RunTerminal  bool
	Depth        int
	MaxDepth     int
	// Ancestors are trusted task IDs in the current delegation lineage.
	Ancestors []string
	TargetTaskID string
	// Existing is the set of requests in this run (all statuses).
	Existing []CoordinationRequest
	TurnCount int
}

// Admit normalizes host-owned fields and enforces replay and limits. A host
// must load and save under a transaction/lease; this function does not write.
func Admit(in Admission, req CoordinationRequest, now time.Time) (CoordinationRequest, bool, error) {
	if !in.Authorized || in.OwnerID == "" || in.RunID == "" || in.SourceTaskID == "" || in.SourceAgent == "" || in.TurnID == "" ||
		in.RunTerminal || now.IsZero() || in.Depth < 0 || in.MaxDepth <= 0 || in.TurnCount < 0 {
		return CoordinationRequest{}, false, ErrCoordinationDenied
	}
	if req.RunID != "" && req.RunID != in.RunID || req.SourcePackageID != "" && req.SourcePackageID != in.SourceTaskID ||
		req.SourceAgentID != "" && req.SourceAgentID != in.SourceAgent {
		return CoordinationRequest{}, false, ErrCoordinationDenied
	}
	req.RunID, req.SourcePackageID, req.SourceAgentID = in.RunID, in.SourceTaskID, in.SourceAgent
	// Only the host controls lifecycle, materialization, receipt and timestamp.
	req.Status, req.CreatedAt = CoordinationPending, now.UTC()
	req.ChildPackageID, req.ResultReceiptID, req.ResolutionSummary = "", "", ""
	req.MaterializedAt, req.ResolvedAt = nil, nil
	derivedID := RequestID(req)
	if req.ID != "" && req.ID != derivedID {
		return CoordinationRequest{}, false, ErrCoordinationConflict
	}
	req.ID = derivedID
	if err := ValidateCoordinationRequest(req); err != nil {
		return CoordinationRequest{}, false, err
	}
	for _, old := range in.Existing {
		if old.ID != req.ID {
			continue
		}
		if old.RunID != in.RunID || !SemanticallyEqual(old, req) {
			return CoordinationRequest{}, false, ErrCoordinationConflict
		}
		return CloneRequest(old), true, nil
	}
	if len(in.Existing) >= MaxRequestsPerRun || in.TurnCount >= MaxRequestsPerTurn {
		return CoordinationRequest{}, false, ErrCoordinationLimit
	}
	if req.Operation == CoordinationDelegate {
		if in.Depth >= in.MaxDepth || in.TargetTaskID == in.SourceTaskID {
			return CoordinationRequest{}, false, ErrCoordinationLimit
		}
		for _, ancestor := range in.Ancestors {
			if ancestor == in.TargetTaskID && in.TargetTaskID != "" {
				return CoordinationRequest{}, false, ErrCoordinationDenied
			}
		}
	}
	return CloneRequest(req), false, nil
}

// Resolve allows exactly one terminal transition; exact replay is idempotent.
// Receipt verification and authorization are host responsibilities.
func Resolve(req CoordinationRequest, status CoordinationStatus, summary, receipt string, authorized bool, now time.Time) (CoordinationRequest, error) {
	if !authorized || now.IsZero() || now.Before(req.CreatedAt) || len(summary) > maxCoordinationObjectiveBytes || len(receipt) > maxCoordinationListItemBytes {
		return CoordinationRequest{}, ErrCoordinationDenied
	}
	switch status {
	case CoordinationResolved, CoordinationRejected, CoordinationFailed, CoordinationCanceled:
	default:
		return CoordinationRequest{}, ErrCoordinationConflict
	}
	summary, receipt = strings.TrimSpace(summary), strings.TrimSpace(receipt)
	if status == CoordinationResolved && (summary == "" || receipt == "") {
		return CoordinationRequest{}, ErrCoordinationConflict
	}
	if req.Status != CoordinationPending {
		if req.Status == status && req.ResolutionSummary == summary && req.ResultReceiptID == receipt {
			return CloneRequest(req), nil
		}
		return CoordinationRequest{}, ErrCoordinationConflict
	}
	out := CloneRequest(req)
	out.Status, out.ResolutionSummary, out.ResultReceiptID = status, summary, receipt
	resolved := now.UTC()
	out.ResolvedAt = &resolved
	return out, nil
}
