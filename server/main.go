// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/main.go — Licensify server entrypoint wiring DB, CA, and HTTP API.

package main

import (
	"context"
	"log"
	"os"

	"github.com/rithulkamesh/licensify/server/internal/api"
	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
)

func main() {
	ctx := context.Background()
	store, err := db.NewStore(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	authority, err := ca.NewAuthority()
	if err != nil {
		log.Fatal(err)
	}
	srv := api.New(store, authority, os.Getenv("LICENSIFY_API_KEY"))
	if err := srv.Echo.Start(":8080"); err != nil {
		log.Fatal(err)
	}
}
