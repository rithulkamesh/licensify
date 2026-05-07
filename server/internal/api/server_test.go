// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/api/server_test.go — Tests for the HTTP API server behavior.

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
)

func TestHealthAndCaArePublic(t *testing.T) {
	ctx := context.Background()
	store, err := db.NewStore(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := ca.NewAuthority()
	if err != nil {
		t.Fatal(err)
	}
	s := New(store, authority, "secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health expected 200 got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/.well-known/ca", nil)
	rec2 := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("ca expected 200 got %d", rec2.Code)
	}
	if rec2.Body.Len() == 0 {
		t.Fatal("ca body empty")
	}
}

func TestProtectedRoutesRequireApiKey(t *testing.T) {
	ctx := context.Background()
	store, _ := db.NewStore(ctx, "")
	authority, _ := ca.NewAuthority()
	s := New(store, authority, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v1/license", nil)
	rec := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/license", nil)
	req2.Header.Set("X-API-Key", "secret")
	rec2 := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 got %d", rec2.Code)
	}
}

