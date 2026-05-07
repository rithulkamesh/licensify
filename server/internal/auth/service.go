// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/auth/service.go — API key authentication and request authorization helpers.

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

type Repository interface {
	StoreOpaqueRecord(context.Context, string, []byte) error
	GetOpaqueRecord(context.Context, string) ([]byte, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, licenseID string, upload []byte) error {
	if len(upload) == 0 {
		return errors.New("opaque upload required")
	}
	return s.repo.StoreOpaqueRecord(ctx, licenseID, upload)
}

func (s *Service) StartLogin(ctx context.Context, licenseID string, request []byte) ([]byte, []byte, error) {
	record, err := s.repo.GetOpaqueRecord(ctx, licenseID)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	h := hmac.New(sha256.New, record)
	h.Write(request)
	h.Write(nonce)
	sessionKey := h.Sum(nil)
	return nonce, sessionKey, nil
}
