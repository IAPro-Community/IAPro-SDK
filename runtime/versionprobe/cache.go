// Package versionprobe coalesces read-only CLI version discovery. It must not
// be used for authentication or admission decisions.
package versionprobe

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"sync"
	"strings"
	"time"
)

type Result struct {
	Installed  bool
	Version    string
	BinaryPath string
	Error      string
}

type Runner func(context.Context, string, []string, []string, string) (string, error)

type key struct {
	binary string
	mod    int64
	size   int64
	input  [32]byte
}

type entry struct {
	result Result
	until  time.Time
}

type flight struct {
	done   chan struct{}
	result Result
}

type Cache struct {
	mu         sync.Mutex
	entries    map[key]entry
	flights    map[key]*flight
	generation uint64
	run        Runner
	now        func() time.Time
}

const maxEntries = 256

func New(run Runner, now func() time.Time) *Cache {
	return &Cache{entries: make(map[key]entry), flights: make(map[key]*flight), run: run, now: now}
}

func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
	clear(c.flights)
	c.generation++
}

func Normalize(binary, output string, err error) Result {
	r := Result{Installed: true, Version: "unknown", BinaryPath: binary}
	if err != nil {
		r.Error = "version probe failed"
	} else if v := strings.TrimSpace(output); v == "" || len(v) > 4096 {
		r.Error = "version probe returned no usable output"
	} else {
		r.Version = v
	}
	return r
}

// Detect accepts host-authorized binary, command, environment and directory.
// Hash inputs instead of storing environment contents or credentials.
func (c *Cache) Detect(ctx context.Context, binary string, args, env []string, dir string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Normalize(binary, "", err)
	}
	if len(args) == 0 {
		args = []string{"--version"}
	}
	args, env = append([]string(nil), args...), append([]string(nil), env...)
	info, err := os.Stat(binary)
	if err != nil || info.IsDir() {
		return Result{BinaryPath: binary, Error: "version probe binary unavailable"}
	}
	input, _ := json.Marshal([]any{args, env, dir})
	k := key{binary: binary, mod: info.ModTime().UnixNano(), size: info.Size(), input: sha256.Sum256(input)}
	clear(input)
	c.mu.Lock()
	now := c.now()
	if e, ok := c.entries[k]; ok && now.Before(e.until) {
		c.mu.Unlock()
		return e.result
	}
	if f, ok := c.flights[k]; ok {
		c.mu.Unlock()
		return wait(ctx, binary, f)
	}
	for k, e := range c.entries {
		if !now.Before(e.until) {
			delete(c.entries, k)
		}
	}
	if len(c.flights) >= maxEntries {
		c.mu.Unlock()
		return Result{Installed: true, BinaryPath: binary, Version: "unknown", Error: "version probe capacity reached"}
	}
	f := &flight{done: make(chan struct{})}
	c.flights[k] = f
	generation := c.generation
	c.mu.Unlock()
	go func() {
		probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := c.run(probeCtx, binary, args, env, dir)
		if probeCtx.Err() != nil {
			err = probeCtx.Err()
		}
		cancel()
		r := Normalize(binary, out, err)
		ttl := 60 * time.Second
		if r.Error != "" {
			ttl = 10 * time.Second
		}
		c.mu.Lock()
		if c.generation == generation && c.flights[k] == f {
			delete(c.flights, k)
			if len(c.entries) < maxEntries {
				c.entries[k] = entry{result: r, until: c.now().Add(ttl)}
			}
		}
		f.result = r
		close(f.done)
		c.mu.Unlock()
	}()
	return wait(ctx, binary, f)
}

func wait(ctx context.Context, binary string, f *flight) Result {
	select {
	case <-ctx.Done():
		return Normalize(binary, "", ctx.Err())
	case <-f.done:
		return f.result
	}
}
