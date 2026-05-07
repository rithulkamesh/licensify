// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/token/token.go — License token generation and verification primitives.

package token

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

type LicenseToken struct {
	Version      uint8
	LicenseID    [16]byte
	MachineID    [32]byte
	IssuedAt     int64
	ExpiresAt    int64
	Entitlements []byte
	Nonce        [32]byte
	Signature    [64]byte
}

func Build(licenseID string, machineID [32]byte, entitlements []byte, ttl time.Duration, signingKey ed25519.PrivateKey) ([]byte, error) {
	id, err := uuid.Parse(licenseID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	tok := LicenseToken{
		Version:      1,
		MachineID:    machineID,
		IssuedAt:     now,
		ExpiresAt:    now + int64(ttl.Seconds()),
		Entitlements: entitlements,
	}
	copy(tok.LicenseID[:], id[:])
	if _, err := rand.Read(tok.Nonce[:]); err != nil {
		return nil, err
	}
	body := marshalUnsigned(tok)
	sig := ed25519.Sign(signingKey, body)
	copy(tok.Signature[:], sig)
	return marshalSigned(tok), nil
}

func marshalUnsigned(t LicenseToken) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(t.Version)
	buf.Write(t.LicenseID[:])
	buf.Write(t.MachineID[:])
	_ = binary.Write(buf, binary.BigEndian, t.IssuedAt)
	_ = binary.Write(buf, binary.BigEndian, t.ExpiresAt)
	_ = binary.Write(buf, binary.BigEndian, uint32(len(t.Entitlements)))
	buf.Write(t.Entitlements)
	buf.Write(t.Nonce[:])
	return buf.Bytes()
}

func marshalSigned(t LicenseToken) []byte {
	u := marshalUnsigned(t)
	return append(u, t.Signature[:]...)
}
