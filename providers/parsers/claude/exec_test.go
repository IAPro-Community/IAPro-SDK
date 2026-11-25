package claude

import (
	"strings"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestClaudeFreshWholeTreeUsage(t *testing.T) {
	var got string
	d := NewExecDecoder(func(e providers.Event) error {
		got += e.Text
		return nil
	})
	root := `{"type":"assistant","session_id":"test","uuid":"one","parent_tool_use_id":null,"message":{"content":[{"type":"text","text":"hello"},{"type":"thinking","thinking":"hidden"}]}}`
	raw := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"test"}`,
		root,
		root,
		`{"type":"assistant","session_id":"test","uuid":"child","parent_tool_use_id":"tool","message":{"content":[{"type":"text","text":"not root"}]}}`,
		`{"type":"stream_event","event":{"delta":{"type":"text_delta","text":"duplicate"}}}`,
		`{"type":"result","subtype":"success","session_id":"test","is_error":false,"result":"hello","usage":{"input_tokens":1,"output_tokens":1},"modelUsage":{"a":{"inputTokens":10,"outputTokens":2,"cacheReadInputTokens":3,"cacheCreationInputTokens":4},"b":{"inputTokens":5,"outputTokens":6,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`,
	}, "\n")
	for i := range raw {
		if _, err := d.Write([]byte(raw[i : i+1])); err != nil {
			t.Fatal(err)
		}
	}
	v, err := d.Finish()
	if err != nil || got != "hello" || v.SessionID != "test" || v.InputTokens == nil || *v.InputTokens != 22 ||
		v.OutputTokens == nil || *v.OutputTokens != 8 {
		t.Fatalf("unexpected result: %+v %v text=%q", v, err, got)
	}
}

func TestClaudeRequiresTerminalEvidence(t *testing.T) {
	prefix := "{\"type\":\"system\",\"subtype\":\"init\",\"session_id\":\"test\"}\n"
	for _, suffix := range []string{
		"",
		`{"type":"result","subtype":"error_during_execution","session_id":"test","is_error":true,"errors":["secret"]}`,
		`{"type":"result","subtype":"success","session_id":"foreign","is_error":false,"result":"hi"}`,
		`{"type":"result","subtype":"success","session_id":"test","result":"hi"}`,
		`{"type":"result","subtype":"success","session_id":"test","is_error":false,"result":"hi","queued_turn_count":1}`,
	} {
		d := NewExecDecoder(nil)
		_, err := d.Write([]byte(prefix + suffix))
		if err == nil {
			_, err = d.Finish()
		}
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected sanitized failure: %v", err)
		}
	}
	d := NewExecDecoder(nil)
	_, _ = d.Write([]byte(prefix + `{"type":"result","subtype":"success","session_id":"test","is_error":false,"result":"hi","usage":{"input_tokens":5,"output_tokens":2}}`))
	v, err := d.Finish()
	if err != nil || v.InputTokens != nil || v.OutputTokens != nil {
		t.Fatalf("top-level usage is not whole-tree evidence: %+v %v", v, err)
	}
}
