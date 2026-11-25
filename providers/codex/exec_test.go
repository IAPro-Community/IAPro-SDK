package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/codex"
)

func fixtureLaunch(mode string) Launch {
	return func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFixtureProcess$")
		cmd.Env = append(os.Environ(), "IAPRO_TEST_FIXTURE="+mode)
		return cmd, nil
	}
}

func TestFixtureProcess(t *testing.T) {
	switch os.Getenv("IAPRO_TEST_FIXTURE") {
	case "exec":
		fmt.Println(`{"type":"thread.started","thread_id":"synthetic"}`)
		fmt.Println(`{"type":"item.completed","item":{"id":"1","type":"agent_message","text":"hello"}}`)
		fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":7,"output_tokens":3}}`)
		os.Exit(0)
	case "rpc":
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			var req struct {
				ID     *uint64 `json:"id"`
				Method string  `json:"method"`
			}
			if json.Unmarshal(scanner.Bytes(), &req) != nil {
				os.Exit(2)
			}
			if req.ID == nil {
				continue
			}
			switch req.Method {
			case "initialize":
				fmt.Printf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"codexHome\":\"synthetic\"}}\n", *req.ID)
			case "account/rateLimits/read":
				fmt.Printf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"accountId\":\"test\",\"rateLimits\":{\"primary\":{\"usedPercent\":40}}}}\n", *req.ID)
			default:
				os.Exit(3)
			}
		}
		os.Exit(0)
	case "hang":
		time.Sleep(time.Minute)
		os.Exit(1)
	}
}

func TestRunExecFixture(t *testing.T) {
	var text string
	result, err := RunExec(context.Background(), fixtureLaunch("exec"), "test", func(e providers.Event) error {
		text += e.Text
		return nil
	})
	if err != nil || text != "hello" || result.OutputTokens == nil || *result.OutputTokens != 3 {
		t.Fatalf("unexpected turn: %+v %v", result, err)
	}
}

func TestStdioQuotaProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rpc, err := StartStdioRPC(ctx, fixtureLaunch("rpc"))
	if err != nil {
		t.Fatal(err)
	}
	scope := model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "account", IdentityVersion: "v1"}
	v, err := parser.Probe(ctx, rpc, scope, "test", "synthetic", func(a, b string) bool { return a == b }, time.Now)
	if err != nil || len(v.Windows) != 1 || v.Windows[0].RemainingPercent == nil || *v.Windows[0].RemainingPercent != 60 {
		t.Fatalf("unexpected quota: %+v %v", v, err)
	}
}

func TestRunExecCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := RunExec(ctx, fixtureLaunch("hang"), "test", nil); err != context.DeadlineExceeded {
		t.Fatalf("cancellation: %v", err)
	}
}
