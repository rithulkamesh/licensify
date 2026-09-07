// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/memstore_test.go — Tests covering MemStore CRUD + opaque record handling.

package db

import (
	"context"
	"testing"

	"github.com/rithulkamesh/licensify/server/internal/license"
)

func TestMemStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	l := license.License{ID: "abc", LicenseType: license.Perpetual}
	if _, err := s.Create(ctx, l); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, l); err == nil {
		t.Fatal("expected duplicate-create error")
	}
	got, err := s.GetByID(ctx, "abc")
	if err != nil || got.ID != "abc" {
		t.Fatalf("get: %v %v", got, err)
	}
	if _, err := s.GetByID(ctx, "missing"); err == nil {
		t.Fatal("expected not found")
	}
	updated := got
	updated.Revoked = true
	if _, err := s.Update(ctx, updated); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, license.License{ID: "missing"}); err == nil {
		t.Fatal("expected update of missing to fail")
	}
}

func TestMemStoreGetByKeyHash(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	kh := []byte{0xAA, 0xBB, 0xCC}
	if _, err := s.Create(ctx, license.License{ID: "k1", KeyHash: kh, LicenseType: license.Perpetual}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetByKeyHash(ctx, kh)
	if err != nil || got.ID != "k1" {
		t.Fatalf("get by key hash: %v %v", got, err)
	}
	if _, err := s.GetByKeyHash(ctx, []byte{0x00}); err == nil {
		t.Fatal("expected not found for unknown key hash")
	}

	// A second license with the same key hash is rejected (mirrors the
	// Postgres UNIQUE constraint).
	if _, err := s.Create(ctx, license.License{ID: "k2", KeyHash: kh}); err == nil {
		t.Fatal("expected duplicate key-hash create to fail")
	}
}

func TestMemStoreOpaque(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	if err := s.StoreOpaqueRecord(ctx, "abc", []byte("rec")); err != nil {
		t.Fatal(err)
	}
	rec, err := s.GetOpaqueRecord(ctx, "abc")
	if err != nil || string(rec) != "rec" {
		t.Fatalf("get opaque: %v %v", rec, err)
	}
	if _, err := s.GetOpaqueRecord(ctx, "missing"); err == nil {
		t.Fatal("expected missing to error")
	}

	// Store opaque after license create should update the License record's OpaqueRecord too.
	_, _ = s.Create(ctx, license.License{ID: "with-record"})
	if err := s.StoreOpaqueRecord(ctx, "with-record", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetByID(ctx, "with-record")
	if string(got.OpaqueRecord) != "payload" {
		t.Fatalf("opaque not propagated: %s", got.OpaqueRecord)
	}
}

func TestMemStoreHealthAndClose(t *testing.T) {
	s := NewMemStore()
	if err := s.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Close()
}

func TestNewStoreReturnsMemWhenDSNEmpty(t *testing.T) {
	s, err := NewStore(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*MemStore); !ok {
		t.Fatalf("expected *MemStore, got %T", s)
	}
	s.Close()
}

func TestNewStorePropagatesPgErr(t *testing.T) {
	// Point at an unreachable DSN to force pgx pool init failure.
	_, err := NewStore(context.Background(), "postgres://invalid:invalid@127.0.0.1:1/none?connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for invalid pg DSN")
	}
}
