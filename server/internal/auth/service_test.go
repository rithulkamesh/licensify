// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/auth/service_test.go — Tests for the auth service register/login flows.

package auth

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	stored map[string][]byte
	storeErr error
	getErr   error
}

func newFake() *fakeRepo {
	return &fakeRepo{stored: map[string][]byte{}}
}

func (f *fakeRepo) StoreOpaqueRecord(_ context.Context, id string, rec []byte) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	f.stored[id] = rec
	return nil
}
func (f *fakeRepo) GetOpaqueRecord(_ context.Context, id string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	r, ok := f.stored[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func TestRegisterRejectsEmptyUpload(t *testing.T) {
	s := NewService(newFake())
	if err := s.Register(context.Background(), "id", nil); err == nil {
		t.Fatal("expected error for empty upload")
	}
}

func TestRegisterStores(t *testing.T) {
	repo := newFake()
	s := NewService(repo)
	if err := s.Register(context.Background(), "id", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if string(repo.stored["id"]) != "payload" {
		t.Fatalf("expected stored payload, got %q", repo.stored["id"])
	}
}

func TestRegisterPropagatesStoreError(t *testing.T) {
	s := NewService(&fakeRepo{stored: map[string][]byte{}, storeErr: errors.New("disk full")})
	if err := s.Register(context.Background(), "id", []byte("x")); err == nil {
		t.Fatal("expected error")
	}
}

func TestStartLoginMissingRecord(t *testing.T) {
	s := NewService(newFake())
	_, _, err := s.StartLogin(context.Background(), "id", []byte("req"))
	if err == nil {
		t.Fatal("expected error when record missing")
	}
}

func TestStartLoginHappyPath(t *testing.T) {
	repo := newFake()
	s := NewService(repo)
	_ = s.Register(context.Background(), "id", []byte("record"))
	nonce, key, err := s.StartLogin(context.Background(), "id", []byte("req"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != 32 {
		t.Fatalf("expected 32-byte nonce, got %d", len(nonce))
	}
	if len(key) == 0 {
		t.Fatal("expected non-empty session key")
	}
	// Different request should yield a different session key (HMAC over input + nonce).
	nonce2, key2, err := s.StartLogin(context.Background(), "id", []byte("req"))
	if err != nil {
		t.Fatal(err)
	}
	// Nonce is random so the keys MUST differ.
	if string(nonce) == string(nonce2) {
		t.Fatal("expected unique nonces")
	}
	if string(key) == string(key2) {
		t.Fatal("expected unique session keys")
	}
}
