// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/store.go — Postgres store setup and query helpers.

package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rithulkamesh/licensify/server/internal/license"
)

type Store struct {
	pool    *pgxpool.Pool
	mu      sync.RWMutex
	records map[string]license.License
	opaque  map[string][]byte
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	var pool *pgxpool.Pool
	var err error
	if dsn != "" {
		pool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			return nil, err
		}
		if err := ensureSchema(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return &Store{
		pool:    pool,
		records: map[string]license.License{},
		opaque:  map[string][]byte{},
	}, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var reg *string
	if err := pool.QueryRow(ctx, "select to_regclass('public.licenses')").Scan(&reg); err != nil {
		return err
	}
	if reg != nil && *reg == "licenses" {
		return nil
	}

	// In Docker we run from /app, so migrations live at /app/migrations.
	// In dev runs, working directory is typically repo-root/server.
	paths := []string{
		filepath.Join("migrations", "000001_init.sql"),
		filepath.Join("server", "migrations", "000001_init.sql"),
		"/app/migrations/000001_init.sql",
	}
	var sqlBytes []byte
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 0 {
			sqlBytes = b
			break
		}
	}
	if len(sqlBytes) == 0 {
		return errors.New("migrations/000001_init.sql not found for bootstrap")
	}
	_, err := pool.Exec(ctx, string(sqlBytes))
	return err
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Create(ctx context.Context, l license.License) (license.License, error) {
	if s.pool != nil {
		entJSON, _ := json.Marshal(l.Entitlements)
		_, err := s.pool.Exec(ctx, "insert into licenses (id,key_hash,opaque_record,license_type,entitlements,max_activations,created_at,expires_at,revoked) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
			l.ID, l.KeyHash, l.OpaqueRecord, string(l.LicenseType), entJSON, l.MaxActivations, l.CreatedAt, l.ExpiresAt, l.Revoked)
		if err != nil {
			return license.License{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[l.ID] = l
	return l, nil
}

func (s *Store) GetByID(_ context.Context, id string) (license.License, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.records[id]
	if !ok {
		return license.License{}, errors.New("license not found")
	}
	return l, nil
}

func (s *Store) Update(_ context.Context, l license.License) (license.License, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[l.ID]; !ok {
		return license.License{}, errors.New("license not found")
	}
	s.records[l.ID] = l
	return l, nil
}

func (s *Store) StoreOpaqueRecord(_ context.Context, licenseID string, record []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opaque[licenseID] = record
	if l, ok := s.records[licenseID]; ok {
		l.OpaqueRecord = record
		s.records[licenseID] = l
	}
	return nil
}

func (s *Store) GetOpaqueRecord(_ context.Context, licenseID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.opaque[licenseID]
	if !ok {
		return nil, errors.New("opaque record not found")
	}
	return rec, nil
}

func (s *Store) Health(ctx context.Context) error {
	if s.pool == nil {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(c)
}
