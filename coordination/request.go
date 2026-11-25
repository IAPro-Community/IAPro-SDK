// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package coordination

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// CoordinationOperation is a closed set of requests an executing Agent may
// submit to the Mission. Agents never spawn providers or mutate the Flow
// directly; MissionRunner remains the execution authority.
type CoordinationOperation string

const (
	CoordinationNeedInfo        CoordinationOperation = "NEED_INFO"
	CoordinationDelegate        CoordinationOperation = "DELEGATE"
	CoordinationArtifactRequest CoordinationOperation = "ARTIFACT_REQUEST"
	CoordinationReviewRequest   CoordinationOperation = "REVIEW_REQUEST"
)

// CoordinationStatus is the durable lifecycle of a coordination request.
type CoordinationStatus string

const (
	CoordinationPending  CoordinationStatus = "PENDING"
	CoordinationResolved CoordinationStatus = "RESOLVED"
	CoordinationRejected CoordinationStatus = "REJECTED"
	CoordinationFailed   CoordinationStatus = "FAILED"
	CoordinationCanceled CoordinationStatus = "CANCELED"

	// These are safety ceilings, not a competing delegation policy. The
	// canonical planner still decides whether delegation is useful; the runner
	// merely prevents provider output from exploding into unbounded child work.
	maxCoordinationRequestsPerTurn    = 3
	maxCoordinationRequestsPerMission = 12
	maxCoordinationSubjectBytes       = 1024
	maxCoordinationObjectiveBytes     = 8192
	maxCoordinationPreferenceBytes    = 256
	maxCoordinationListItems          = 16
	maxCoordinationListItemBytes      = 1024
)

// CoordinationRequest is a Mission-owned request raised by an executing Agent.
// It is intentionally provider-neutral: source/target are Agent/task concepts,
// while provider/profile/model selection stays with ResourceScheduler.
type CoordinationRequest struct {
	ID                   string                `json:"id"`
	RunID                string                `json:"run_id"`
	SourcePackageID      string                `json:"source_package_id"`
	SourceAgentID        string                `json:"source_agent_id,omitempty"`
	Operation            CoordinationOperation `json:"operation"`
	Subject              string                `json:"subject"`
	Objective            string                `json:"objective,omitempty"`
	PreferredRole        string                `json:"preferred_role,omitempty"`
	PreferredProvider    string                `json:"preferred_provider,omitempty"`
	RequiredCapabilities []string              `json:"required_capabilities,omitempty"`
	ArtifactRefs         []string              `json:"artifact_refs,omitempty"`
	Status               CoordinationStatus    `json:"status"`
	ChildPackageID       string                `json:"child_package_id,omitempty"`
	MaterializedAt       *time.Time            `json:"materialized_at,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	ResolvedAt           *time.Time            `json:"resolved_at,omitempty"`
	ResultReceiptID      string                `json:"result_receipt_id,omitempty"`
	ResolutionSummary    string                `json:"resolution_summary,omitempty"`
}

func validateCoordinationString(name, value string, maxBytes int) error {
	if len(value) > maxBytes {
		return fmt.Errorf("coordination %s exceeds %d bytes", name, maxBytes)
	}
	return nil
}

func validateCoordinationList(name string, values []string) error {
	if len(values) > maxCoordinationListItems {
		return fmt.Errorf("coordination %s exceeds %d items", name, maxCoordinationListItems)
	}
	for _, value := range values {
		if len(value) > maxCoordinationListItemBytes {
			return fmt.Errorf("coordination %s item exceeds %d bytes", name, maxCoordinationListItemBytes)
		}
	}
	return nil
}

// ValidateCoordinationRequest enforces the architectural boundary before a
// request can become durable Mission state.
func ValidateCoordinationRequest(req CoordinationRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("coordination request id is required")
	}
	if strings.TrimSpace(req.RunID) == "" {
		return fmt.Errorf("coordination run_id is required")
	}
	if strings.TrimSpace(req.SourcePackageID) == "" {
		return fmt.Errorf("coordination source_package_id is required")
	}
	for _, id := range []string{req.ID, req.RunID, req.SourcePackageID, req.SourceAgentID} {
		if len(id) > maxCoordinationListItemBytes {
			return fmt.Errorf("coordination identity exceeds %d bytes", maxCoordinationListItemBytes)
		}
	}
	switch req.Operation {
	case CoordinationNeedInfo, CoordinationDelegate, CoordinationArtifactRequest, CoordinationReviewRequest:
	default:
		return fmt.Errorf("unsupported coordination operation %q", req.Operation)
	}
	if strings.TrimSpace(req.Subject) == "" {
		return fmt.Errorf("coordination subject is required")
	}
	if req.Operation == CoordinationDelegate && strings.TrimSpace(req.Objective) == "" {
		return fmt.Errorf("DELEGATE coordination requires objective")
	}
	for name, value := range map[string]string{
		"subject":            req.Subject,
		"objective":          req.Objective,
		"preferred_role":     req.PreferredRole,
		"preferred_provider": req.PreferredProvider,
	} {
		limit := maxCoordinationPreferenceBytes
		switch name {
		case "subject":
			limit = maxCoordinationSubjectBytes
		case "objective":
			limit = maxCoordinationObjectiveBytes
		}
		if err := validateCoordinationString(name, value, limit); err != nil {
			return err
		}
	}
	if err := validateCoordinationList("required_capabilities", req.RequiredCapabilities); err != nil {
		return err
	}
	if err := validateCoordinationList("artifact_refs", req.ArtifactRefs); err != nil {
		return err
	}
	switch req.Status {
	case CoordinationPending, CoordinationResolved, CoordinationRejected, CoordinationFailed, CoordinationCanceled:
	default:
		return fmt.Errorf("unsupported coordination status %q", req.Status)
	}
	return nil
}

func SemanticallyEqual(left, right CoordinationRequest) bool {
	return left.RunID == right.RunID &&
		left.SourcePackageID == right.SourcePackageID &&
		left.SourceAgentID == right.SourceAgentID &&
		left.Operation == right.Operation &&
		left.Subject == right.Subject &&
		left.Objective == right.Objective &&
		left.PreferredRole == right.PreferredRole &&
		left.PreferredProvider == right.PreferredProvider &&
		slices.Equal(left.RequiredCapabilities, right.RequiredCapabilities) &&
		slices.Equal(left.ArtifactRefs, right.ArtifactRefs)
}

func CloneRequest(in CoordinationRequest) CoordinationRequest {
	out := in
	out.RequiredCapabilities = append([]string(nil), in.RequiredCapabilities...)
	out.ArtifactRefs = append([]string(nil), in.ArtifactRefs...)
	if in.MaterializedAt != nil {
		value := *in.MaterializedAt
		out.MaterializedAt = &value
	}
	if in.ResolvedAt != nil {
		value := *in.ResolvedAt
		out.ResolvedAt = &value
	}
	return out
}

// RequestID derives the upstream-compatible semantic identifier. The host must
// also compare the semantic payload and namespace IDs by owner and run.
func RequestID(req CoordinationRequest) string {
	payload, _ := json.Marshal(struct {
		RunID                string                `json:"run_id"`
		SourcePackageID      string                `json:"source_package_id"`
		SourceAgentID        string                `json:"source_agent_id,omitempty"`
		Operation            CoordinationOperation `json:"operation"`
		Subject              string                `json:"subject"`
		Objective            string                `json:"objective,omitempty"`
		PreferredRole        string                `json:"preferred_role,omitempty"`
		PreferredProvider    string                `json:"preferred_provider,omitempty"`
		RequiredCapabilities []string              `json:"required_capabilities,omitempty"`
		ArtifactRefs         []string              `json:"artifact_refs,omitempty"`
	}{
		RunID: req.RunID, SourcePackageID: req.SourcePackageID, SourceAgentID: req.SourceAgentID,
		Operation: req.Operation, Subject: strings.TrimSpace(req.Subject), Objective: strings.TrimSpace(req.Objective),
		PreferredRole: strings.TrimSpace(req.PreferredRole), PreferredProvider: strings.TrimSpace(req.PreferredProvider),
		RequiredCapabilities: req.RequiredCapabilities, ArtifactRefs: req.ArtifactRefs,
	})
	sum := sha256.Sum256(payload)
	return "coordreq_" + hex.EncodeToString(sum[:8])
}

// Limits are ceilings; hosts must enforce them transactionally.
const MaxRequestsPerTurn = maxCoordinationRequestsPerTurn
const MaxRequestsPerRun = maxCoordinationRequestsPerMission
