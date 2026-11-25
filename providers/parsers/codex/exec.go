package codex

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
	"github.com/IAPro-Community/IAPro-SDK/providers/jsonl"
)

const maxEventBytes = jsonl.MaxEventBytes

var ErrIncompleteTurn = errors.New("codex turn lacks successful terminal event")
var ErrInvalidEvent = errors.New("invalid codex JSONL event")

// ExecDecoder implements io.Writer for codex exec --json. It is single-writer,
// emits completed assistant messages (not token deltas), and does not log raw
// CLI output, commands, prompts or diagnostic text.
type ExecDecoder struct {
	framer    *jsonl.Framer
	emit      func(providers.Event) error
	result    providers.RunResult
	completed bool
	messages  map[[32]byte][32]byte
}

func NewExecDecoder(emit func(providers.Event) error) *ExecDecoder {
	d := &ExecDecoder{emit: emit, messages: make(map[[32]byte][32]byte)}
	d.framer = jsonl.New(d.consume)
	return d
}

func (d *ExecDecoder) Write(p []byte) (int, error) {
	return d.framer.Write(p)
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
	if d.completed {
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
			if e.Usage.InputTokens != nil && *e.Usage.InputTokens >= 0 && e.Usage.OutputTokens != nil && *e.Usage.OutputTokens >= 0 && *e.Usage.InputTokens <= math.MaxInt64-*e.Usage.OutputTokens {
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
	if err := d.framer.Finish(); err != nil {
		return d.result, err
	}
	if !d.completed || len(d.messages) == 0 || d.result.SessionID == "" {
		return d.result, ErrIncompleteTurn
	}
	return d.result, nil
}
