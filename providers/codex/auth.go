// SPDX-License-Identifier: MIT

package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const maxAuthMetadata = 4 * 1024 * 1024
const maxStatusOutput = 64 * 1024

// ExternalAccountID extracts only account attribution from the provider's
// auth document. The host reads this document through its authorized secret
// boundary; its contents are never logged or cached by the SDK. The returned
// identifier is NOT proof of an active login or a verified remote identity.
func ExternalAccountID(raw []byte) (string, error) {
	if len(raw) == 0 || len(raw) > maxAuthMetadata {
		return "", errors.New("invalid Codex auth metadata")
	}
	var auth struct {
		Tokens struct {
			AccountID string `json:"account_id"`
		} `json:"tokens"`
	}
	if json.Unmarshal(raw, &auth) != nil || strings.TrimSpace(auth.Tokens.AccountID) == "" {
		return "", errors.New("Codex account identity unavailable")
	}
	return strings.TrimSpace(auth.Tokens.AccountID), nil
}

type statusBuffer struct {
	buffer bytes.Buffer
}

func (b *statusBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := maxStatusOutput - b.buffer.Len(); left > 0 {
		if left < len(p) {
			p = p[:left]
		}
		_, _ = b.buffer.Write(p)
	}
	return n, nil
}

// CheckLoginStatus runs an authorized codex login status command, never login,
// logout or token refresh. Unknown output is an explicit error, not a guess.
func CheckLoginStatus(ctx context.Context, launch Launch) (bool, error) {
	if launch == nil {
		return false, ErrLaunch
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd, err := launch(ctx)
	if err != nil || cmd == nil || cmd.Process != nil {
		return false, ErrLaunch
	}
	var stdout, stderr statusBuffer
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = 2 * time.Second
	err = cmd.Run()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if cmd.Process == nil {
		return false, ErrLaunch
	}
	output := strings.ToLower(stdout.buffer.String() + "\n" + stderr.buffer.String())
	if strings.Contains(output, "not logged in") || strings.Contains(output, "logged out") {
		return false, nil
	}
	if err == nil && strings.Contains(output, "logged in") {
		return true, nil
	}
	return false, ErrProcess
}

var _ io.Writer = (*statusBuffer)(nil)
