// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/go/licensify/client_test.go — Tests for the Licensify Go SDK client wrapper.

package licensify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRejectsEmptyServerURL(t *testing.T) {
	_, err := New(Config{ServerURL: "", CachePath: "/tmp/x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*InitializationError); !ok {
		t.Fatalf("expected *InitializationError, got %T", err)
	}
}

func TestNewRejectsEmptyCachePath(t *testing.T) {
	_, err := New(Config{ServerURL: "x", CachePath: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*InitializationError); !ok {
		t.Fatalf("expected *InitializationError, got %T", err)
	}
}

func TestErrorTypesImplementError(t *testing.T) {
	cases := []error{
		&InitializationError{Message: "init"},
		&ActivationError{Code: ErrActivation, Message: "act"},
		&CheckError{Code: ErrCheck, Message: "chk"},
	}
	for _, e := range cases {
		if e.Error() == "" {
			t.Fatalf("expected non-empty Error() for %T", e)
		}
	}
}

func TestActivateAndCheckOnClosedClient(t *testing.T) {
	c := &Client{}
	if err := c.Activate("k"); err == nil {
		t.Fatal("expected error on closed client")
	}
	if err := c.Activate(""); err == nil {
		t.Fatal("expected error on empty key (and closed client)")
	}
	if _, err := c.Check(); err == nil {
		t.Fatal("expected error on closed client")
	}
	if c.HasFeature("base") {
		t.Fatal("expected false on closed client")
	}
	if c.HasFeature("") {
		t.Fatal("expected false on empty feature")
	}
	if msg := c.lastError(); msg == "" {
		t.Fatal("expected fallback message")
	}
	c.Close()
}

// stubLogger captures invocations to ensure the SDK calls every level.
type stubLogger struct {
	calls map[string]int
}

func (s *stubLogger) Debug(msg string, _ map[string]any) { s.calls["debug"]++ }
func (s *stubLogger) Info(msg string, _ map[string]any)  { s.calls["info"]++ }
func (s *stubLogger) Warn(msg string, _ map[string]any)  { s.calls["warn"]++ }
func (s *stubLogger) Error(msg string, _ map[string]any) { s.calls["error"]++ }

// TestNativeHappyPath requires the cdylib to be built and findable. CI provides this.
// Skipped if LICENSIFY_NATIVE is unset to keep developer machines green.
func TestNativeHappyPath(t *testing.T) {
	if os.Getenv("LICENSIFY_NATIVE") == "" {
		t.Skip("set LICENSIFY_NATIVE=1 to run native tests")
	}
	tmp := t.TempDir()
	logger := &stubLogger{calls: map[string]int{}}
	c, err := New(Config{
		ServerURL: "http://127.0.0.1:0",
		CachePath: filepath.Join(tmp, "licensify.token"),
		Logger:    logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Activate("LICENSE-KEY-DEV"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := c.Activate(""); err == nil {
		t.Fatal("expected error on empty key")
	}
	st, err := c.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if st.Code < 0 {
		t.Fatal("unexpected negative status")
	}
	if !c.HasFeature("base") {
		t.Fatal("expected base feature after activate")
	}
	if c.HasFeature("nope") {
		t.Fatal("expected no nope feature")
	}
	c.Close()
	c.Close() // idempotent
	if logger.calls["info"] == 0 {
		t.Fatal("expected info logs from successful activate")
	}
}
