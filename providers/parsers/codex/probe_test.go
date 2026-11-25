package codex

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

type fixtureRPC struct {
	calls  []string
	home   string
	closed bool
}

func (r *fixtureRPC) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	r.calls = append(r.calls, method)
	if method == "initialize" {
		return json.Marshal(map[string]string{"codexHome": r.home})
	}
	return json.RawMessage(`{"accountId":"test","rateLimits":{"primary":{"usedPercent":40}}}`), nil
}

func (r *fixtureRPC) Notify(_ context.Context, method string, _ any) error {
	r.calls = append(r.calls, method)
	return nil
}

func (r *fixtureRPC) Close() error {
	r.closed = true
	return nil
}

func TestProbeReadOnlyAndIdentity(t *testing.T) {
	scope := model.AccountScope{ProviderID: "codex", ProfileID: "p", AccountID: "a", IdentityVersion: "v1"}
	same := func(a, b string) bool { return a == b }
	rpc := &fixtureRPC{home: "/profile"}
	v, err := Probe(context.Background(), rpc, scope, "test", "/profile", same, time.Now)
	if err != nil || v.AccountScope != scope || !rpc.closed || !reflect.DeepEqual(rpc.calls, []string{"initialize", "initialized", "account/rateLimits/read"}) {
		t.Fatalf("unexpected probe: %+v %v %+v", v, err, rpc)
	}
	rpc = &fixtureRPC{home: "/foreign"}
	if _, err := Probe(context.Background(), rpc, scope, "test", "/profile", same, time.Now); err == nil || len(rpc.calls) != 1 || !rpc.closed {
		t.Fatal("foreign home should stop before quota call")
	}
}
