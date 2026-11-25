package gemini

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestGeminiFixture(t *testing.T) {
	if os.Getenv("IAPRO_GEMINI_FIXTURE") == "1" {
		fmt.Println(`{"type":"init","session_id":"synthetic"}`)
		fmt.Println(`{"type":"message","role":"assistant","content":"hello","delta":true}`)
		fmt.Println(`{"type":"result","status":"success","stats":{"input_tokens":7,"output_tokens":3}}`)
		os.Exit(0)
	}
	var got string
	result, err := RunExec(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGeminiFixture$")
		cmd.Env = append(os.Environ(), "IAPRO_GEMINI_FIXTURE=1")
		return cmd, nil
	}, "synthetic prompt", func(e providers.Event) error {
		got += e.Text
		return nil
	})
	if err != nil || got != "hello" || result.OutputTokens == nil || *result.OutputTokens != 3 {
		t.Fatalf("unexpected fixture result: %+v %v", result, err)
	}
}
