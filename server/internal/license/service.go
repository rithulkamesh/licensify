// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/license/service.go — License lifecycle operations (create/update/activate/deactivate/validate).

package license

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(context.Context, License) (License, error)
	GetByID(context.Context, string) (License, error)
	Update(context.Context, License) (License, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func HashLicenseKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

func (s *Service) Create(ctx context.Context, key string, lt LicenseType, ent Entitlements, expiresAt *time.Time) (License, error) {
	if key == "" {
		return License{}, errors.New("license key required")
	}
	if ent.OfflineGraceDays == 0 {
		ent.OfflineGraceDays = 7
	}
	l := License{
		ID:           uuid.NewString(),
		KeyHash:      HashLicenseKey(key),
		OpaqueRecord: []byte{},
		LicenseType:  lt,
		Entitlements: ent,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    expiresAt,
	}
	return s.repo.Create(ctx, l)
}

func (s *Service) Get(ctx context.Context, id string) (License, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, l License) (License, error) {
	return s.repo.Update(ctx, l)
}

func StableMachineID(machineID []byte) string {
	return hex.EncodeToString(machineID)
}
