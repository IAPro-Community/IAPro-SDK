// Package jsonl implements bounded, single-writer event framing shared by CLI adapters.
package jsonl

import (
	"bytes"
	"errors"
)

const MaxEventBytes = 4 * 1024 * 1024
const MaxEvents = 100000
const MaxStreamBytes = 128 * 1024 * 1024

var ErrInvalid = errors.New("invalid or oversized JSONL stream")

// Framer bounds memory, total input, and event count. Errors are sticky;
// no further provider events are delivered after an invalid frame.
type Framer struct {
	pending  []byte
	consume  func([]byte) error
	err      error
	count    int
	total    int
	finished bool
}

func New(consume func([]byte) error) *Framer {
	return &Framer{consume: consume}
}

func (f *Framer) Write(p []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.finished || f.consume == nil || len(p) > MaxStreamBytes-f.total {
		f.err = ErrInvalid
		return 0, f.err
	}
	f.total += len(p)
	n := len(p)
	for len(p) > 0 {
		line, rest, end := bytes.Cut(p, []byte{'\n'})
		if len(line) > MaxEventBytes-len(f.pending) {
			f.err = ErrInvalid
			return n - len(p), f.err
		}
		f.pending = append(f.pending, line...)
		if !end {
			break
		}
		if err := f.deliver(); err != nil {
			return n - len(rest), err
		}
		p = rest
	}
	return n, nil
}

func (f *Framer) deliver() error {
	if len(bytes.TrimSpace(f.pending)) != 0 {
		f.count++
		if f.count > MaxEvents || f.consume == nil {
			f.err = ErrInvalid
		} else {
			f.err = f.consume(f.pending)
		}
	}
	f.pending = f.pending[:0]
	return f.err
}

// Finish delivers the final unterminated line, if present. A second call is
// harmless; a previously failed stream cannot be made successful.
func (f *Framer) Finish() error {
	if f.finished || f.err != nil {
		return f.err
	}
	f.finished = true
	if len(f.pending) != 0 {
		return f.deliver()
	}
	return nil
}
