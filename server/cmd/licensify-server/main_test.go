// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/cmd/licensify-server/main_test.go — Tests for the cmd entrypoint via injectable seams.

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/rithulkamesh/licensify/server/internal/server"
)

func TestEntrypointUsesDefaultAddrAndPropagatesError(t *testing.T) {
	origRun, origFatal := runFn, fatalFn
	defer func() { runFn, fatalFn = origRun, origFatal }()

	var seenAddr string
	runFn = func(_ context.Context, opts server.Options) error {
		seenAddr = opts.Addr
		return errors.New("boom")
	}
	var fatalErr error
	fatalFn = func(args ...any) {
		if len(args) > 0 {
			if e, ok := args[0].(error); ok {
				fatalErr = e
			}
		}
	}

	entrypoint(func(string) string { return "" })
	if seenAddr != ":8080" {
		t.Fatalf("expected default addr, got %s", seenAddr)
	}
	if fatalErr == nil {
		t.Fatal("expected fatal to be invoked with an error")
	}
}

func TestEntrypointHonorsListenAddrEnv(t *testing.T) {
	origRun, origFatal := runFn, fatalFn
	defer func() { runFn, fatalFn = origRun, origFatal }()

	var seenAddr string
	runFn = func(_ context.Context, opts server.Options) error {
		seenAddr = opts.Addr
		return nil
	}
	fatalFn = func(args ...any) {}

	entrypoint(func(k string) string {
		if k == "LICENSIFY_LISTEN_ADDR" {
			return ":7777"
		}
		return ""
	})
	if seenAddr != ":7777" {
		t.Fatalf("expected :7777, got %s", seenAddr)
	}
}

func TestEntrypointHappyPathDoesNotCallFatal(t *testing.T) {
	origRun, origFatal := runFn, fatalFn
	defer func() { runFn, fatalFn = origRun, origFatal }()

	runFn = func(_ context.Context, _ server.Options) error { return nil }
	fatalCalled := false
	fatalFn = func(args ...any) { fatalCalled = true }
	entrypoint(func(string) string { return "" })
	if fatalCalled {
		t.Fatal("fatalFn must not be called on happy path")
	}
}
