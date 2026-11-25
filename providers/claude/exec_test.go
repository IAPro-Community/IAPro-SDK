package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestFreshExecFixture(t *testing.T) {
	if os.Getenv("IAPRO_CLAUDE_FIXTURE") == "1" {
		fmt.Println(`{"type":"system","subtype":"init","session_id":"synthetic"}`)
		fmt.Println(`{"type":"assistant","session_id":"synthetic","uuid":"one","message":{"content":[{"type":"text","text":"hello"}]}}`)
		fmt.Println(`{"type":"result","subtype":"success","session_id":"synthetic","is_error":false,"modelUsage":{"fixture":{"inputTokens":7,"outputTokens":3,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`)
		os.Exit(0)
	}
	var got string
	v, err := RunFreshExec(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFreshExecFixture$")
		cmd.Env = append(os.Environ(), "IAPRO_CLAUDE_FIXTURE=1")
		return cmd, nil
	}, "fixture", func(e providers.Event) error {
		got += e.Text
		return nil
	})
	if err != nil || got != "hello" || v.OutputTokens == nil || *v.OutputTokens != 3 {
		t.Fatalf("unexpected result: %+v %v", v, err)
	}
}
