package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
)

const maxEventBytes = 4 * 1024 * 1024
const maxEvents = 100000

var ErrIncompleteTurn = errors.New("codex turn lacks successful terminal event")
var ErrInvalidEvent = errors.New("invalid codex JSONL event")

// ExecDecoder implements io.Writer for codex exec --json. It is single-writer,
// emits completed assistant messages (not token deltas), and does not log raw
// CLI output, commands, prompts or diagnostic text.
type ExecDecoder struct {
	pending   []byte
	emit      func(providers.Event) error
	result    providers.RunResult
	completed bool
	failed    bool
	count     int
	messages  map[[32]byte][32]byte
}

func NewExecDecoder(emit func(providers.Event) error) *ExecDecoder {
	return &ExecDecoder{emit: emit, messages: make(map[[32]byte][32]byte)}
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

type execEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Item     *struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Usage *struct {
		InputTokens  *int64 `json:"input_tokens"`
		OutputTokens *int64 `json:"output_tokens"`
	} `json:"usage"`
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
	case "thread.started":
		if e.ThreadID == "" || (d.result.SessionID != "" && d.result.SessionID != e.ThreadID) {
			return ErrInvalidEvent
		}
		d.result.SessionID = e.ThreadID
	case "item.completed":
		if e.Item == nil {
			return ErrInvalidEvent
		}
		if e.Item.Type != "agent_message" || e.Item.Text == "" {
			return nil
		}
		if e.Item.ID == "" {
			return ErrInvalidEvent
		}
		id := sha256.Sum256([]byte(e.Item.ID))
		text := sha256.Sum256([]byte(e.Item.Text))
		if previous, ok := d.messages[id]; ok {
			if previous != text {
				return ErrInvalidEvent
			}
			return nil
		}
		d.messages[id] = text
		if d.emit != nil {
			return d.emit(providers.Event{Kind: providers.EventText, Text: e.Item.Text})
		}
	case "turn.completed":
		if e.Usage != nil {
			if e.Usage.InputTokens != nil && *e.Usage.InputTokens >= 0 && e.Usage.OutputTokens != nil && *e.Usage.OutputTokens >= 0 {
				d.result.InputTokens = e.Usage.InputTokens
				d.result.OutputTokens = e.Usage.OutputTokens
			}
		}
		d.completed = true
	case "turn.failed", "error":
		d.result.Failure = model.Failure{Kind: model.FailureUnknown, Message: "Codex reported a failed turn"}
		return ErrIncompleteTurn
	}
	return nil
}

// Finish must be called after the process exits. A successful JSONL terminal
// event is necessary but not sufficient: the host must also verify exit status.
func (d *ExecDecoder) Finish() (providers.RunResult, error) {
	if len(d.pending) != 0 && !d.failed {
		if err := d.consume(d.pending); err != nil {
			d.failed = true
			return d.result, err
		}
		d.pending = nil
	}
	if d.failed || !d.completed || len(d.messages) == 0 {
		return d.result, ErrIncompleteTurn
	}
	return d.result, nil
}
