package jsonl

import (
	"errors"
	"strings"
	"testing"
)

func TestFramerSplitAndFinalLine(t *testing.T) {
	var got []string
	f := New(func(line []byte) error {
		got = append(got, string(line))
		return nil
	})
	for _, chunk := range []string{"first", "\n\nsec", "ond", "\nlast"} {
		if n, err := f.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("write: %d %v", n, err)
		}
	}
	if err := f.Finish(); err != nil || strings.Join(got, ",") != "first,second,last" {
		t.Fatalf("frames: %v %v", got, err)
	}
	if err := f.Finish(); err != nil || len(got) != 3 {
		t.Fatal("finish must be idempotent")
	}
}

func TestFramerStickyFailureAndLimits(t *testing.T) {
	sentinel := errors.New("bad event")
	calls := 0
	f := New(func([]byte) error {
		calls++
		return sentinel
	})
	if _, err := f.Write([]byte("bad\nother\n")); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("retry\n")); !errors.Is(err, sentinel) || calls != 1 {
		t.Fatal("failed stream resumed")
	}
	if err := f.Finish(); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	f = New(func([]byte) error { return nil })
	if _, err := f.Write(make([]byte, MaxEventBytes+1)); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized event accepted")
	}
}
