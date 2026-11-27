package codex

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestClassifyRPCError(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		auth bool
	}{
		{`{"code":401,"message":"private diagnostic"}`, true},
		{`{"code":-32000,"message":"invalid_jwt private diagnostic"}`, true},
		{`{"code":-32000,"message":"request failed","data":{"error":"token_expired"}}`, true},
		{`{"code":-32000,"message":"request id 4017"}`, false},
		{`{"code":-32000,"message":"unknown error private diagnostic"}`, false},
		{`not JSON`, false},
	} {
		err := classifyRPCError(json.RawMessage(tc.raw))
		if tc.auth {
			if providers.ClassifyProbeError(err) != providers.ProbeAuthExpired || err.Error() != string(providers.ProbeAuthExpired) {
				t.Fatalf("unexpected auth failure: %v", err)
			}
		} else if !errors.Is(err, ErrProtocol) {
			t.Fatalf("expected protocol error: %v", err)
		}
	}
}
