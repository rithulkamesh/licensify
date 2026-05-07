// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/memstore.go — In-memory implementation of Store used in tests and DSN-less runs.

package db

import (
	"context"
	"errors"
	"sync"

	"github.com/rithulkamesh/licensify/server/internal/license"
)

// MemStore is a thread-safe in-memory implementation of Store. It is used when
// no DATABASE_URL is provided, and from every server-level test so that we
// never need a running Postgres for unit tests.
type MemStore struct {
	mu      sync.RWMutex
	records map[string]license.License
	opaque  map[string][]byte
}

func NewMemStore() *MemStore {
	return &MemStore{
		records: map[string]license.License{},
		opaque:  map[string][]byte{},
	}
}

func (s *MemStore) Create(_ context.Context, l license.License) (license.License, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[l.ID]; ok {
		return license.License{}, errors.New("license already exists")
	}
	s.records[l.ID] = l
	return l, nil
}

func (s *MemStore) GetByID(_ context.Context, id string) (license.License, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.records[id]
	if !ok {
		return license.License{}, errors.New("license not found")
	}
	return l, nil
}

func (s *MemStore) Update(_ context.Context, l license.License) (license.License, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[l.ID]; !ok {
		return license.License{}, errors.New("license not found")
	}
	s.records[l.ID] = l
	return l, nil
}

func (s *MemStore) StoreOpaqueRecord(_ context.Context, licenseID string, record []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opaque[licenseID] = record
	if l, ok := s.records[licenseID]; ok {
		l.OpaqueRecord = record
		s.records[licenseID] = l
	}
	return nil
}

func (s *MemStore) GetOpaqueRecord(_ context.Context, licenseID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.opaque[licenseID]
	if !ok {
		return nil, errors.New("opaque record not found")
	}
	return rec, nil
}

func (s *MemStore) Health(_ context.Context) error { return nil }

func (s *MemStore) Close() {}
