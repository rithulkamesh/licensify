// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/license/service_test.go — Tests for license lifecycle and helper functions.

package license

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	created   License
	createErr error
	getErr    error
	updateErr error
}

func (f *fakeRepo) Create(_ context.Context, l License) (License, error) {
	if f.createErr != nil {
		return License{}, f.createErr
	}
	f.created = l
	return l, nil
}
func (f *fakeRepo) GetByID(_ context.Context, id string) (License, error) {
	if f.getErr != nil {
		return License{}, f.getErr
	}
	if f.created.ID == id {
		return f.created, nil
	}
	return License{}, errors.New("not found")
}
func (f *fakeRepo) GetByKeyHash(_ context.Context, keyHash []byte) (License, error) {
	if f.getErr != nil {
		return License{}, f.getErr
	}
	if f.created.KeyHash != nil && bytes.Equal(f.created.KeyHash, keyHash) {
		return f.created, nil
	}
	return License{}, errors.New("not found")
}
func (f *fakeRepo) Update(_ context.Context, l License) (License, error) {
	if f.updateErr != nil {
		return License{}, f.updateErr
	}
	f.created = l
	return l, nil
}

func TestCreateRejectsEmptyKey(t *testing.T) {
	s := NewService(&fakeRepo{})
	_, err := s.Create(context.Background(), "", Perpetual, Entitlements{}, nil)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCreateAppliesDefaultGraceDays(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	l, err := s.Create(context.Background(), "key", Perpetual, Entitlements{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if l.Entitlements.OfflineGraceDays != 7 {
		t.Fatalf("expected default grace 7, got %d", l.Entitlements.OfflineGraceDays)
	}
	if len(l.KeyHash) != 32 {
		t.Fatalf("expected 32-byte key hash, got %d", len(l.KeyHash))
	}
	if l.ID == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestCreateRespectsExplicitGrace(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	exp := time.Unix(1_700_000_000, 0).UTC()
	l, err := s.Create(context.Background(), "key", Subscription, Entitlements{OfflineGraceDays: 14}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	if l.Entitlements.OfflineGraceDays != 14 {
		t.Fatalf("expected 14, got %d", l.Entitlements.OfflineGraceDays)
	}
	if l.ExpiresAt == nil || !l.ExpiresAt.Equal(exp) {
		t.Fatal("expires not propagated")
	}
}

func TestCreatePropagatesRepoError(t *testing.T) {
	s := NewService(&fakeRepo{createErr: errors.New("boom")})
	_, err := s.Create(context.Background(), "key", Perpetual, Entitlements{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetAndUpdateDelegate(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	created, _ := s.Create(context.Background(), "k", Perpetual, Entitlements{}, nil)
	got, err := s.Get(context.Background(), created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get: %v %v", got, err)
	}
	got.Revoked = true
	upd, err := s.Update(context.Background(), got)
	if err != nil || !upd.Revoked {
		t.Fatalf("update: %v %v", upd, err)
	}
}

func TestGetByKey(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	if _, err := s.GetByKey(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
	created, _ := s.Create(context.Background(), "the-key", Perpetual, Entitlements{}, nil)
	got, err := s.GetByKey(context.Background(), "the-key")
	if err != nil || got.ID != created.ID {
		t.Fatalf("get by key: %v %v", got, err)
	}
	if _, err := s.GetByKey(context.Background(), "other-key"); err == nil {
		t.Fatal("expected not found for unknown key")
	}
}

func TestHashLicenseKeyDeterministic(t *testing.T) {
	a := HashLicenseKey("k")
	b := HashLicenseKey("k")
	c := HashLicenseKey("z")
	if string(a) != string(b) {
		t.Fatal("expected deterministic hash")
	}
	if string(a) == string(c) {
		t.Fatal("hash collision for different keys")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32-byte hash, got %d", len(a))
	}
}

func TestStableMachineID(t *testing.T) {
	id := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	got := StableMachineID(id)
	if got != "deadbeef" {
		t.Fatalf("expected hex, got %s", got)
	}
}

func TestEntitlementsToJSON(t *testing.T) {
	e := Entitlements{LicenseType: Perpetual, Features: []string{"a"}, OfflineGraceDays: 1}
	b, err := e.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("expected non-empty json")
	}
}
