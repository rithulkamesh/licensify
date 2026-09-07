// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/db/pgstore.go — Postgres implementation of Store with bootstrap migration handling.

package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rithulkamesh/licensify/server/internal/license"
)

// PgStore is the production-grade implementation of Store backed by Postgres.
type PgStore struct {
	pool *pgxpool.Pool
}

// migrationCandidatePaths is the search list for the bootstrap SQL migration.
// In Docker the working directory is /app, in local dev it is repo-root/server.
var migrationCandidatePaths = []string{
	filepath.Join("migrations", "000001_init.sql"),
	filepath.Join("server", "migrations", "000001_init.sql"),
	"/app/migrations/000001_init.sql",
}

// readMigration is a seam so tests can inject migration content without touching the filesystem.
var readMigration = func() ([]byte, error) {
	for _, p := range migrationCandidatePaths {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, errors.New("migrations/000001_init.sql not found for bootstrap")
}

// NewPgStore opens a pool against dsn and ensures the schema exists.
func NewPgStore(ctx context.Context, dsn string) (*PgStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgStore{pool: pool}, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var reg *string
	if err := pool.QueryRow(ctx, "select to_regclass('public.licenses')").Scan(&reg); err != nil {
		return err
	}
	if reg != nil && *reg == "licenses" {
		return nil
	}
	sqlBytes, err := readMigration()
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		return err
	}
	return nil
}

func (s *PgStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *PgStore) Create(ctx context.Context, l license.License) (license.License, error) {
	entJSON, _ := json.Marshal(l.Entitlements)
	_, err := s.pool.Exec(ctx,
		"insert into licenses (id,key_hash,opaque_record,license_type,entitlements,max_activations,created_at,expires_at,revoked) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		l.ID, l.KeyHash, l.OpaqueRecord, string(l.LicenseType), entJSON, l.MaxActivations, l.CreatedAt, l.ExpiresAt, l.Revoked)
	if err != nil {
		return license.License{}, err
	}
	return l, nil
}

func (s *PgStore) GetByID(ctx context.Context, id string) (license.License, error) {
	var l license.License
	var entJSON []byte
	var lt string
	err := s.pool.QueryRow(ctx,
		"select id, key_hash, opaque_record, license_type, entitlements, max_activations, created_at, expires_at, revoked from licenses where id=$1", id,
	).Scan(&l.ID, &l.KeyHash, &l.OpaqueRecord, &lt, &entJSON, &l.MaxActivations, &l.CreatedAt, &l.ExpiresAt, &l.Revoked)
	if err != nil {
		return license.License{}, err
	}
	l.LicenseType = license.LicenseType(lt)
	if err := json.Unmarshal(entJSON, &l.Entitlements); err != nil {
		return license.License{}, err
	}
	return l, nil
}

func (s *PgStore) GetByKeyHash(ctx context.Context, keyHash []byte) (license.License, error) {
	var l license.License
	var entJSON []byte
	var lt string
	err := s.pool.QueryRow(ctx,
		"select id, key_hash, opaque_record, license_type, entitlements, max_activations, created_at, expires_at, revoked from licenses where key_hash=$1", keyHash,
	).Scan(&l.ID, &l.KeyHash, &l.OpaqueRecord, &lt, &entJSON, &l.MaxActivations, &l.CreatedAt, &l.ExpiresAt, &l.Revoked)
	if err != nil {
		return license.License{}, err
	}
	l.LicenseType = license.LicenseType(lt)
	if err := json.Unmarshal(entJSON, &l.Entitlements); err != nil {
		return license.License{}, err
	}
	return l, nil
}

func (s *PgStore) Update(ctx context.Context, l license.License) (license.License, error) {
	entJSON, _ := json.Marshal(l.Entitlements)
	tag, err := s.pool.Exec(ctx,
		"update licenses set license_type=$2, entitlements=$3, expires_at=$4, revoked=$5 where id=$1",
		l.ID, string(l.LicenseType), entJSON, l.ExpiresAt, l.Revoked)
	if err != nil {
		return license.License{}, err
	}
	if tag.RowsAffected() == 0 {
		return license.License{}, errors.New("license not found")
	}
	return l, nil
}

func (s *PgStore) StoreOpaqueRecord(ctx context.Context, licenseID string, record []byte) error {
	_, err := s.pool.Exec(ctx,
		"update licenses set opaque_record=$2 where id=$1", licenseID, record)
	return err
}

func (s *PgStore) GetOpaqueRecord(ctx context.Context, licenseID string) ([]byte, error) {
	var rec []byte
	err := s.pool.QueryRow(ctx,
		"select opaque_record from licenses where id=$1", licenseID,
	).Scan(&rec)
	if err != nil {
		return nil, err
	}
	if len(rec) == 0 {
		return nil, errors.New("opaque record not found")
	}
	return rec, nil
}

func (s *PgStore) Health(ctx context.Context) error {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(c)
}
