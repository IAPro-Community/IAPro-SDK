package versionprobe

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalescingAndInvalidation(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "cli")
	if err := os.WriteFile(bin, []byte("v1"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	c := New(func(context.Context, string, []string, []string, string) (string, error) {
		calls.Add(1)
		return "cli 1.0", nil
	}, time.Now)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := c.Detect(context.Background(), bin, nil, []string{"HOME=a"}, ""); got.Version != "cli 1.0" {
				t.Errorf("version: %+v", got)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	c.Detect(context.Background(), bin, nil, []string{"HOME=b"}, "")
	if err := os.WriteFile(bin, []byte("upgraded"), 0600); err != nil {
		t.Fatal(err)
	}
	c.Detect(context.Background(), bin, nil, []string{"HOME=b"}, "")
	c.Invalidate()
	c.Detect(context.Background(), bin, nil, []string{"HOME=b"}, "")
	if calls.Load() != 4 {
		t.Fatalf("calls=%d; environment, binary and invalidation must refresh", calls.Load())
	}
}
