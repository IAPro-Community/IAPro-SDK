package claude

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
	"github.com/IAPro-Community/IAPro-SDK/providers/jsonl"
)

var ErrInvalidEvent = errors.New("invalid Claude Code JSONL event")
var ErrIncompleteTurn = errors.New("Claude Code turn lacks successful terminal evidence")

// ExecDecoder supports single-shot, fresh Claude Code sessions only. It
// consumes complete root assistant messages; partial events, tool payloads,
// subagents and raw diagnostics are not forwarded.
type ExecDecoder struct {
	framer    *jsonl.Framer
	emit      func(providers.Event) error
	result    providers.RunResult
	messages  map[[32]byte][32]byte
	emitted   bool
	completed bool
}

func NewExecDecoder(emit func(providers.Event) error) *ExecDecoder {
	d := &ExecDecoder{emit: emit, messages: make(map[[32]byte][32]byte)}
	d.framer = jsonl.New(d.consume)
	return d
}

func (d *ExecDecoder) Write(p []byte) (int, error) {
	return d.framer.Write(p)
}

type modelUsage struct {
	InputTokens        *int64 `json:"inputTokens"`
	OutputTokens       *int64 `json:"outputTokens"`
	CacheReadInput     *int64 `json:"cacheReadInputTokens"`
	CacheCreationInput *int64 `json:"cacheCreationInputTokens"`
}

type execEvent struct {
	Type            string                `json:"type"`
	Subtype         string                `json:"subtype"`
	SessionID       string                `json:"session_id"`
	UUID            string                `json:"uuid"`
	ParentToolUseID *string               `json:"parent_tool_use_id"`
	AgentID         string                `json:"agent_id"`
	IsError         *bool                 `json:"is_error"`
	Aborted         bool                  `json:"aborted"`
	Supersedes      []string              `json:"supersedes"`
	Error           string                `json:"error"`
	Result          string                `json:"result"`
	QueuedTurns     int                   `json:"queued_turn_count"`
	DeferredToolUse json.RawMessage       `json:"deferred_tool_use"`
	ModelUsage      map[string]modelUsage `json:"modelUsage"`
	Message         *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
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
	case "system":
		if e.Subtype == "init" {
			if e.SessionID == "" || d.result.SessionID != "" {
				return ErrInvalidEvent
			}
			d.result.SessionID = e.SessionID
		}
	case "assistant":
		if e.ParentToolUseID != nil || e.AgentID != "" {
			return nil
		}
		if d.result.SessionID == "" || e.SessionID != d.result.SessionID || e.UUID == "" || e.Message == nil || e.Aborted || len(e.Supersedes) != 0 {
			return ErrInvalidEvent
		}
		if e.Error != "" {
			return ErrIncompleteTurn
		}
		id, hash := sha256.Sum256([]byte(e.UUID)), sha256.Sum256(line)
		if previous, ok := d.messages[id]; ok {
			if previous != hash {
				return ErrInvalidEvent
			}
			return nil
		}
		d.messages[id] = hash
		for _, block := range e.Message.Content {
			if block.Type == "text" && block.Text != "" {
				if err := d.emitText(block.Text); err != nil {
					return err
				}
			}
		}
	case "result":
		if d.result.SessionID == "" || e.SessionID != d.result.SessionID ||
			e.Subtype != "success" || e.IsError == nil || *e.IsError || e.QueuedTurns != 0 ||
			(len(e.DeferredToolUse) != 0 && string(e.DeferredToolUse) != "null") {
			d.result.Failure = model.Failure{Kind: model.FailureUnknown, Message: "Claude Code reported an incomplete turn"}
			return ErrIncompleteTurn
		}
		// Only a successful fresh single-shot result can establish whole-tree
		// counters. Failed results may omit spend; resumed totals are cumulative.
		if input, output, ok := totalUsage(e.ModelUsage); ok {
			d.result.InputTokens, d.result.OutputTokens = &input, &output
		}
		if !d.emitted && e.Result != "" {
			if err := d.emitText(e.Result); err != nil {
				return err
			}
		}
		d.completed = true
	}
	return nil
}

func (d *ExecDecoder) emitText(text string) error {
	d.emitted = true
	if d.emit != nil {
		return d.emit(providers.Event{Kind: providers.EventText, Text: text})
	}
	return nil
}

// totalUsage uses per-model counters because top-level usage excludes subagent
// spend. Cache read and creation counts contribute to token volume. We do not
// use CLI estimated USD values as billing data.
func totalUsage(models map[string]modelUsage) (input, output int64, ok bool) {
	if len(models) == 0 {
		return 0, 0, false
	}
	add := func(total *int64, n *int64) bool {
		if n == nil || *n < 0 || *n > math.MaxInt64-*total {
			return false
		}
		*total += *n
		return true
	}
	for name, usage := range models {
		if name == "" || !add(&input, usage.InputTokens) || !add(&input, usage.CacheReadInput) ||
			!add(&input, usage.CacheCreationInput) || !add(&output, usage.OutputTokens) {
			return 0, 0, false
		}
	}
	if input > math.MaxInt64-output {
		return 0, 0, false
	}
	return input, output, true
}

func (d *ExecDecoder) Finish() (providers.RunResult, error) {
	if err := d.framer.Finish(); err != nil {
		return d.result, err
	}
	if !d.completed || !d.emitted {
		return d.result, ErrIncompleteTurn
	}
	return d.result, nil
}
