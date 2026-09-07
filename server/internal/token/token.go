// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/token/token.go — License token generation and verification primitives.

package token

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SigningKeyEnv is the environment variable holding a hex- or base64-encoded
// 32-byte Ed25519 seed used to sign license tokens. It must be stable across
// restarts so previously issued offline tokens keep verifying; when unset a
// fresh random key is generated (fine for tests and ephemeral dev servers).
const SigningKeyEnv = "LICENSIFY_TOKEN_SIGNING_KEY"

// SigningKeyFromEnv returns the token-signing key. If `env(SigningKeyEnv)` holds
// a valid 32-byte seed (hex or standard base64) that seed is used; otherwise a
// new random key is generated.
func SigningKeyFromEnv(env func(string) string) (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(env(SigningKeyEnv))
	if raw == "" {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		return key, err
	}
	seed, err := decodeSeed(raw)
	if err != nil {
		return nil, err
	}
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("token signing seed must be 32 bytes")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func decodeSeed(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// PublicKeyHex returns the lowercase hex encoding of the key's public half —
// the value the client verifies offline tokens against.
func PublicKeyHex(key ed25519.PrivateKey) string {
	return hex.EncodeToString(key.Public().(ed25519.PublicKey))
}

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
