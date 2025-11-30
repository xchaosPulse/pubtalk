package pactl

import (
	"errors"
	"testing"
)

type fakeExec struct {
	lastName string
	lastArgs []string
	out      []byte
	err      error
}

func (f *fakeExec) Run(name string, args ...string) ([]byte, error) {
	f.lastName = name
	f.lastArgs = args
	return f.out, f.err
}

func TestPactlDryRun(t *testing.T) {
	c := New(true)
	out, err := c.Pactl("list", "sinks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "dry-run: pactl list sinks" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestPactlExecForwarding(t *testing.T) {
	fx := &fakeExec{out: []byte("OK"), err: nil}
	c := &Client{Exec: fx, DryRun: false}
	out, err := c.Pactl("list", "short", "sinks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "OK" {
		t.Fatalf("unexpected output: %s", out)
	}
	if fx.lastName != "pactl" {
		t.Fatalf("expected pactl command, got %s", fx.lastName)
	}
	if len(fx.lastArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(fx.lastArgs))
	}
}

func TestPactlExecErrorWrapped(t *testing.T) {
	fx := &fakeExec{out: []byte("err out"), err: errors.New("failed")}
	c := &Client{Exec: fx, DryRun: false}
	_, err := c.Pactl("some", "cmd")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestListSinkInputs(t *testing.T) {
	fx := &fakeExec{out: []byte(`Sink Input #0
	application.name = "Firefox"
Sink Input #1
	application.name = "Chrome"`), err: nil}
	c := &Client{Exec: fx, DryRun: false}
	inputs, err := c.ListSinkInputs()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(inputs) == 0 {
		t.Fatal("expected at least one sink input")
	}
}
