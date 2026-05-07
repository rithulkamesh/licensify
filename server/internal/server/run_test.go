// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/server/run_test.go — Tests for the test-friendly Run() entrypoint.

package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/rithulkamesh/licensify/server/internal/api"
	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
)

func TestRunRequiresEnv(t *testing.T) {
	if err := Run(context.Background(), Options{}); err == nil {
		t.Fatal("expected error when env is nil")
	}
}

func TestRunReturnsServerClosedAsNil(t *testing.T) {
	called := false
	err := Run(context.Background(), Options{
		Env:          func(string) string { return "" },
		NewStore:     func(_ context.Context, _ string) (db.Store, error) { return db.NewMemStore(), nil },
		NewAuthority: func() (*ca.Authority, error) { return ca.NewAuthority() },
		Listener: func(s *api.Server, addr string) error {
			called = true
			if addr != ":8080" {
				t.Fatalf("expected default addr, got %s", addr)
			}
			return http.ErrServerClosed
		},
	})
	if err != nil {
		t.Fatalf("expected nil for ErrServerClosed, got %v", err)
	}
	if !called {
		t.Fatal("listener not invoked")
	}
}

func TestRunPropagatesListenerError(t *testing.T) {
	err := Run(context.Background(), Options{
		Env:          func(string) string { return "" },
		Addr:         "127.0.0.1:0",
		NewStore:     func(_ context.Context, _ string) (db.Store, error) { return db.NewMemStore(), nil },
		NewAuthority: func() (*ca.Authority, error) { return ca.NewAuthority() },
		Listener: func(*api.Server, string) error {
			return errors.New("listener-boom")
		},
	})
	if err == nil || err.Error() != "listener-boom" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRunPropagatesStoreError(t *testing.T) {
	err := Run(context.Background(), Options{
		Env: func(string) string { return "" },
		NewStore: func(_ context.Context, _ string) (db.Store, error) {
			return nil, errors.New("store-boom")
		},
		Listener: func(*api.Server, string) error { return nil },
	})
	if err == nil || err.Error() != "store-boom" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRunPropagatesCAError(t *testing.T) {
	err := Run(context.Background(), Options{
		Env:      func(string) string { return "" },
		NewStore: func(_ context.Context, _ string) (db.Store, error) { return db.NewMemStore(), nil },
		NewAuthority: func() (*ca.Authority, error) {
			return nil, errors.New("ca-boom")
		},
		Listener: func(*api.Server, string) error { return nil },
	})
	if err == nil || err.Error() != "ca-boom" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRunUsesDefaultsWhenOptionsOmitted(t *testing.T) {
	// Provide only Env so the function exercises every default branch.
	err := Run(context.Background(), Options{
		Env: func(string) string { return "" },
		Listener: func(*api.Server, string) error {
			return http.ErrServerClosed
		},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
