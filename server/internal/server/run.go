// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/server/run.go — Test-friendly Run() that wires the server and starts Echo on a configurable address.

package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/rithulkamesh/licensify/server/internal/api"
	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
	"github.com/rithulkamesh/licensify/server/internal/token"
)

// Env mirrors the os.Getenv signature so tests can inject configuration without
// touching the process environment.
type Env func(key string) string

// Options bundle the dependencies that vary between production and tests.
type Options struct {
	Env  Env
	Addr string
	// NewStore lets tests inject an in-memory or fake store. When nil, the
	// default factory is used (Postgres if DATABASE_URL is set, in-memory otherwise).
	NewStore func(ctx context.Context, dsn string) (db.Store, error)
	// NewAuthority lets tests inject a deterministic CA. When nil, the default
	// `ca.NewAuthority` is used.
	NewAuthority func() (*ca.Authority, error)
	// Listener is the function used to start serving. When nil, `srv.Echo.Start(addr)` is used.
	Listener func(server *api.Server, addr string) error
}

// Run constructs and starts the server. It returns nil if the listener exits
// cleanly with http.ErrServerClosed (used by tests that gracefully shut down).
func Run(ctx context.Context, opts Options) error {
	if opts.Env == nil {
		return errors.New("env lookup is required")
	}
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}
	newStore := opts.NewStore
	if newStore == nil {
		newStore = db.NewStore
	}
	newAuthority := opts.NewAuthority
	if newAuthority == nil {
		newAuthority = ca.NewAuthority
	}
	listener := opts.Listener
	if listener == nil {
		listener = func(server *api.Server, addr string) error {
			return server.Echo.Start(addr)
		}
	}

	store, err := newStore(ctx, opts.Env("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	authority, err := newAuthority()
	if err != nil {
		return err
	}

	tokenKey, err := token.SigningKeyFromEnv(opts.Env)
	if err != nil {
		return err
	}

	srv := api.New(store, authority, opts.Env("LICENSIFY_API_KEY"), tokenKey)
	if err := listener(srv, opts.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
