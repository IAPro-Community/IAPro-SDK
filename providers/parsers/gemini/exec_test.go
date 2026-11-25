package gemini

import (
	"strings"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestGeminiExecDecoder(t *testing.T) {
	var got string
	d := NewExecDecoder(func(e providers.Event) error {
		got += e.Text
		return nil
	})
	raw := strings.Join([]string{
		`{"type":"init","session_id":"synthetic","model":"fixture"}`,
		`{"type":"message","role":"user","content":"private prompt"}`,
		`{"type":"message","role":"assistant","content":"hel","delta":true}`,
		`{"type":"tool_result","output":"private tool output"}`,
		`{"type":"message","role":"assistant","content":"lo","delta":true}`,
		`{"type":"result","status":"success","stats":{"input_tokens":12,"output_tokens":3}}`,
	}, "\n")
	for i := range raw {
		if _, err := d.Write([]byte(raw[i : i+1])); err != nil {
			t.Fatal(err)
		}
	}
	result, err := d.Finish()
	if err != nil || got != "hello" || result.SessionID != "synthetic" || result.InputTokens == nil || *result.InputTokens != 12 ||
		result.OutputTokens == nil || *result.OutputTokens != 3 {
		t.Fatalf("unexpected result: %+v %v text=%q", result, err, got)
	}
}

func TestGeminiTerminalEvidence(t *testing.T) {
	prefix := "{\"type\":\"init\",\"session_id\":\"test\"}\n{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"hi\"}\n"
	for _, suffix := range []string{
		"",
		`{"type":"result","status":"error","error":{"message":"sensitive"}}`,
		`{"type":"result","stats":{"input_tokens":12,"output_tokens":3}}`,
		`{"type":"result","status":"success"}` + "\n" + `{"type":"message","role":"assistant","content":"late"}`,
	} {
		d := NewExecDecoder(nil)
		_, err := d.Write([]byte(prefix + suffix))
		if err == nil {
			_, err = d.Finish()
		}
		if err == nil || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("expected sanitized protocol failure: %v", err)
		}
	}
	d := NewExecDecoder(nil)
	_, _ = d.Write([]byte(prefix + `{"type":"result","status":"success","stats":{"input_tokens":-1,"output_tokens":3}}`))
	result, err := d.Finish()
	if err != nil || result.InputTokens != nil || result.OutputTokens != nil {
		t.Fatalf("invalid counters must remain unknown: %+v %v", result, err)
	}
}
