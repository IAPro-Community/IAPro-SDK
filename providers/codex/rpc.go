// SPDX-License-Identifier: MIT

package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

const maxRPCLine = 4 * 1024 * 1024
const maxRPCMessages = 512

// StdioRPC owns a single transient app-server. The host must hold the
// account's maintenance lease and pass an isolated, trusted launch function.
type StdioRPC struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	in     io.WriteCloser
	out    io.ReadCloser
	reader *bufio.Reader
	mu     sync.Mutex
	once   sync.Once
	nextID uint64
}

func StartStdioRPC(ctx context.Context, launch Launch) (*StdioRPC, error) {
	if launch == nil || ctx.Err() != nil {
		return nil, ErrLaunch
	}
	runCtx, cancel := context.WithCancel(ctx)
	cmd, err := launch(runCtx)
	if err != nil || cmd == nil || cmd.Process != nil {
		cancel()
		return nil, ErrLaunch
	}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, ErrLaunch
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		cancel()
		return nil, ErrLaunch
	}
	if err := cmd.Start(); err != nil {
		_ = in.Close()
		_ = out.Close()
		cancel()
		return nil, ErrLaunch
	}
	return &StdioRPC{cmd: cmd, cancel: cancel, in: in, out: out, reader: bufio.NewReaderSize(out, maxRPCLine)}, nil
}

// Close cancels and reaps the owned process. No diagnostic payload is exposed.
func (r *StdioRPC) Close() error {
	r.once.Do(func() {
		r.cancel()
		_ = r.in.Close()
		_ = r.out.Close()
		_ = r.cmd.Wait()
	})
	return nil
}

type rpcResponse struct {
	ID     *uint64         `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (r *StdioRPC) write(method string, params any, id uint64) error {
	payload := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		payload["params"] = params
	}
	if id != 0 {
		payload["id"] = id
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) >= maxRPCLine {
		return ErrProtocol
	}
	if _, err := r.in.Write(append(data, '\n')); err != nil {
		return ErrProtocol
	}
	return nil
}

type rpcOutcome struct {
	raw json.RawMessage
	err error
}

// exchange serializes requests. Cancellation closes the entire connection,
// so a late response cannot be misattributed to a later account request.
func (r *StdioRPC) exchange(ctx context.Context, method string, params any, notify bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(chan rpcOutcome, 1)
	go func() {
		id := uint64(0)
		if !notify {
			r.nextID++
			id = r.nextID
		}
		if err := r.write(method, params, id); err != nil {
			result <- rpcOutcome{err: err}
			return
		}
		if notify {
			result <- rpcOutcome{}
			return
		}
		for i := 0; i < maxRPCMessages; i++ {
			line, err := r.reader.ReadSlice('\n')
			if err != nil {
				result <- rpcOutcome{err: ErrProtocol}
				return
			}
			var response rpcResponse
			if json.Unmarshal(line, &response) != nil {
				result <- rpcOutcome{err: ErrProtocol}
				return
			}
			if response.Method != "" && response.ID != nil {
				// A quota probe must not execute server-initiated requests.
				result <- rpcOutcome{err: errors.New("unexpected app-server request")}
				return
			}
			if response.ID == nil || *response.ID != id {
				continue
			}
			if len(response.Error) > 0 && string(response.Error) != "null" {
				result <- rpcOutcome{err: classifyRPCError(response.Error)}
				return
			}
			if len(response.Result) == 0 {
				result <- rpcOutcome{err: ErrProtocol}
				return
			}
			result <- rpcOutcome{raw: response.Result}
			return
		}
		result <- rpcOutcome{err: ErrProtocol}
	}()
	select {
	case <-ctx.Done():
		_ = r.Close()
		<-result
		return nil, ctx.Err()
	case out := <-result:
		if out.err != nil {
			_ = r.Close()
		}
		return out.raw, out.err
	}
}

func (r *StdioRPC) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return r.exchange(ctx, method, params, false)
}

func (r *StdioRPC) Notify(ctx context.Context, method string, params any) error {
	_, err := r.exchange(ctx, method, params, true)
	return err
}

// classifyRPCError accepts only the matched JSON-RPC error object. Provider
// diagnostics may contain secrets; return a fixed code, never raw text.
func classifyRPCError(raw json.RawMessage) error {
	var failure struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if len(raw) > 64*1024 || json.Unmarshal(raw, &failure) != nil {
		return ErrProtocol
	}
	detail := strings.ToLower(failure.Message + " " + string(failure.Data))
	if failure.Code == 401 || strings.Contains(detail, "invalid_jwt") ||
		strings.Contains(detail, "unauthorized") || strings.Contains(detail, "token_expired") ||
		strings.Contains(detail, "token expired") || strings.Contains(detail, "http 401") ||
		strings.Contains(detail, "status 401") {
		return &providers.ProbeError{Code: providers.ProbeAuthExpired}
	}
	return ErrProtocol
}
