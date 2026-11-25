package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
)

const maxEventBytes = 4 * 1024 * 1024
const maxEvents = 100000

var ErrInvalidEvent = errors.New("invalid Gemini JSONL event")
var ErrIncompleteTurn = errors.New("Gemini turn lacks a successful result")

// ExecDecoder consumes Gemini CLI stream-json output. Only assistant message
// content is emitted; prompts, tool arguments, tool output and raw errors are
// not forwarded. Stats are used only from the terminal result.
type ExecDecoder struct {
	pending   []byte
	emit      func(providers.Event) error
	result    providers.RunResult
	count     int
	emitted   bool
	completed bool
	failed    bool
}

func NewExecDecoder(emit func(providers.Event) error) *ExecDecoder {
	return &ExecDecoder{emit: emit}
}

type execEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	Stats     *struct {
		InputTokens  *int64 `json:"input_tokens"`
		OutputTokens *int64 `json:"output_tokens"`
	} `json:"stats"`
}

func (d *ExecDecoder) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		line, rest, end := bytes.Cut(p, []byte{'\n'})
		if len(d.pending)+len(line) > maxEventBytes {
			d.failed = true
			return 0, ErrInvalidEvent
		}
		d.pending = append(d.pending, line...)
		if !end {
			break
		}
		if err := d.consume(d.pending); err != nil {
			d.failed = true
			return 0, err
		}
		d.pending = d.pending[:0]
		p = rest
	}
	return n, nil
}

func (d *ExecDecoder) consume(line []byte) error {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil
	}
	d.count++
	if d.count > maxEvents || d.completed || d.failed {
		return ErrInvalidEvent
	}
	var e execEvent
	if json.Unmarshal(line, &e) != nil || e.Type == "" {
		return ErrInvalidEvent
	}
	switch e.Type {
	case "init":
		if e.SessionID == "" || (d.result.SessionID != "" && d.result.SessionID != e.SessionID) {
			return ErrInvalidEvent
		}
		d.result.SessionID = e.SessionID
	case "message":
		if e.Role != "assistant" || e.Content == "" {
			return nil
		}
		if d.result.SessionID == "" {
			return ErrInvalidEvent
		}
		d.emitted = true
		if d.emit != nil {
			return d.emit(providers.Event{Kind: providers.EventText, Text: e.Content})
		}
	case "result":
		if e.Stats != nil && e.Stats.InputTokens != nil && e.Stats.OutputTokens != nil {
			in, out := *e.Stats.InputTokens, *e.Stats.OutputTokens
			if in >= 0 && out >= 0 && in <= math.MaxInt64-out {
				d.result.InputTokens = &in
				d.result.OutputTokens = &out
			}
		}
		if e.Status != "success" {
			d.result.Failure = model.Failure{Kind: model.FailureUnknown, Message: "Gemini reported an unsuccessful result"}
			return ErrIncompleteTurn
		}
		d.completed = true
	}
	return nil
}

// Finish requires a successful terminal event and at least one assistant chunk.
// A nonzero process exit is checked separately by the shared runtime.
func (d *ExecDecoder) Finish() (providers.RunResult, error) {
	if len(d.pending) != 0 && !d.failed {
		if err := d.consume(d.pending); err != nil {
			d.failed = true
			return d.result, err
		}
		d.pending = nil
	}
	if d.failed || !d.completed || !d.emitted || d.result.SessionID == "" {
		return d.result, ErrIncompleteTurn
	}
	return d.result, nil
}
