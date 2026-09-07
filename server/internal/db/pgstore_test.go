// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/pgstore_test.go — Postgres-backed store tests gated on LICENSIFY_TEST_DATABASE_URL.

package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rithulkamesh/licensify/server/internal/license"
)

func pgTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("LICENSIFY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LICENSIFY_TEST_DATABASE_URL not set; skipping pgstore tests")
	}
	return dsn
}

func TestNewPgStoreFailsForBadDSN(t *testing.T) {
	if _, err := NewPgStore(context.Background(), "postgres://invalid:invalid@127.0.0.1:1/none?connect_timeout=1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPgStoreCRUDAndOpaque(t *testing.T) {
	dsn := pgTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := NewPgStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Health(ctx); err != nil {
		t.Fatal(err)
	}

	l := license.License{
		ID:           uuid.NewString(),
		KeyHash:      []byte{1, 2, 3},
		OpaqueRecord: []byte{},
		LicenseType:  license.Perpetual,
		Entitlements: license.Entitlements{LicenseType: license.Perpetual, Features: []string{"a"}, OfflineGraceDays: 7},
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := s.Create(ctx, l); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByID(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != l.ID {
		t.Fatal("id mismatch")
	}

	if _, err := s.GetByID(ctx, uuid.NewString()); err == nil {
		t.Fatal("expected not found")
	}

	byKey, err := s.GetByKeyHash(ctx, l.KeyHash)
	if err != nil || byKey.ID != l.ID {
		t.Fatalf("get by key hash: %v %v", byKey, err)
	}
	if _, err := s.GetByKeyHash(ctx, []byte{0xFF, 0xFE}); err == nil {
		t.Fatal("expected not found for unknown key hash")
	}

	got.Revoked = true
	if _, err := s.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, license.License{ID: uuid.NewString(), Entitlements: l.Entitlements}); err == nil {
		t.Fatal("expected update-missing error")
	}

	if err := s.StoreOpaqueRecord(ctx, l.ID, []byte("rec")); err != nil {
		t.Fatal(err)
	}
	rec, err := s.GetOpaqueRecord(ctx, l.ID)
	if err != nil || string(rec) != "rec" {
		t.Fatalf("opaque: %v %v", rec, err)
	}
	if _, err := s.GetOpaqueRecord(ctx, uuid.NewString()); err == nil {
		t.Fatal("expected missing opaque error")
	}
}

func TestReadMigrationFallsBackThroughPaths(t *testing.T) {
	// Save and restore the readMigration seam.
	orig := readMigration
	defer func() { readMigration = orig }()
	readMigration = func() ([]byte, error) {
		return nil, errors.New("simulate-not-found")
	}
	if _, err := readMigration(); err == nil {
		t.Fatal("expected injected error")
	}
}
