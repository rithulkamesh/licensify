// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/store.go — Store interface and factory selecting an in-memory or Postgres implementation.

package db

import (
	"context"

	"github.com/rithulkamesh/licensify/server/internal/license"
)

// Store is the persistence interface used by the server. It has two implementations:
// - MemStore (always available, used in tests and when DATABASE_URL is empty)
// - PgStore (used when DATABASE_URL is set)
type Store interface {
	Create(ctx context.Context, l license.License) (license.License, error)
	GetByID(ctx context.Context, id string) (license.License, error)
	GetByKeyHash(ctx context.Context, keyHash []byte) (license.License, error)
	Update(ctx context.Context, l license.License) (license.License, error)
	StoreOpaqueRecord(ctx context.Context, licenseID string, record []byte) error
	GetOpaqueRecord(ctx context.Context, licenseID string) ([]byte, error)
	Health(ctx context.Context) error
	Close()
}

// NewStore returns a Store backed by Postgres if dsn is non-empty, otherwise an
// in-memory store. The function is the only construction seam: callers should
// not import the concrete types directly.
func NewStore(ctx context.Context, dsn string) (Store, error) {
	if dsn == "" {
		return NewMemStore(), nil
	}
	return NewPgStore(ctx, dsn)
}
