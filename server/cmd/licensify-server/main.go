// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/cmd/licensify-server/main.go — CLI entrypoint delegating wiring to internal/server.Run for testability.

package main

import (
	"context"
	"log"
	"os"

	"github.com/rithulkamesh/licensify/server/internal/server"
)

// runFn is a seam for tests so they can swap server.Run with a fake.
var runFn = server.Run

// fatalFn is a seam for tests so log.Fatal does not call os.Exit.
var fatalFn = log.Fatal

func entrypoint(env func(string) string) {
	addr := env("LICENSIFY_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if err := runFn(context.Background(), server.Options{
		Env:  env,
		Addr: addr,
	}); err != nil {
		fatalFn(err)
	}
}

func main() {
	entrypoint(os.Getenv)
}
