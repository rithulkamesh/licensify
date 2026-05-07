// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/token/token_test.go — Tests for token generation/verification behavior.

package token

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"
)

func parseUnsigned(t *testing.T, b []byte) (unsigned []byte, sig []byte) {
	t.Helper()
	if len(b) < 64 {
		t.Fatalf("token too short: %d", len(b))
	}
	unsigned = b[:len(b)-64]
	sig = b[len(b)-64:]
	return unsigned, sig
}

func TestBuildAndVerifySignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var mid [32]byte
	copy(mid[:], bytes.Repeat([]byte{0xAB}, 32))
	tok, err := Build("11111111-1111-1111-1111-111111111111", mid, []byte{1, 2, 3}, 10*time.Second, priv)
	if err != nil {
		t.Fatal(err)
	}
	unsigned, sig := parseUnsigned(t, tok)
	if !ed25519.Verify(pub, unsigned, sig) {
		t.Fatal("signature did not verify")
	}
	if unsigned[0] != 1 {
		t.Fatalf("unexpected version: %d", unsigned[0])
	}
	at := 1 + 16 + 32 + 8 + 8
	entLen := binary.BigEndian.Uint32(unsigned[at : at+4])
	if entLen != 3 {
		t.Fatalf("unexpected ent len: %d", entLen)
	}
}

func TestBuildRejectsInvalidUUID(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	var mid [32]byte
	if _, err := Build("not-a-uuid", mid, nil, time.Second, priv); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarshalUnsignedFieldOffsets(t *testing.T) {
	tok := LicenseToken{
		Version:      1,
		IssuedAt:     1700000000,
		ExpiresAt:    1700000060,
		Entitlements: []byte("hello"),
	}
	copy(tok.LicenseID[:], bytes.Repeat([]byte{0x10}, 16))
	copy(tok.MachineID[:], bytes.Repeat([]byte{0x20}, 32))
	copy(tok.Nonce[:], bytes.Repeat([]byte{0x30}, 32))
	out := marshalUnsigned(tok)
	if out[0] != 1 {
		t.Fatal("version offset wrong")
	}
	if !bytes.Equal(out[1:17], bytes.Repeat([]byte{0x10}, 16)) {
		t.Fatal("license id offset wrong")
	}
	if !bytes.Equal(out[17:49], bytes.Repeat([]byte{0x20}, 32)) {
		t.Fatal("machine id offset wrong")
	}
	if binary.BigEndian.Uint64(out[49:57]) != 1700000000 {
		t.Fatal("issued_at wrong")
	}
	if binary.BigEndian.Uint64(out[57:65]) != 1700000060 {
		t.Fatal("expires_at wrong")
	}
	if binary.BigEndian.Uint32(out[65:69]) != 5 {
		t.Fatal("ent len wrong")
	}
	if string(out[69:74]) != "hello" {
		t.Fatal("entitlements wrong")
	}
	if !bytes.Equal(out[74:106], bytes.Repeat([]byte{0x30}, 32)) {
		t.Fatal("nonce offset wrong")
	}
}

func TestMarshalSignedAppendsSig(t *testing.T) {
	tok := LicenseToken{Version: 1}
	copy(tok.Signature[:], bytes.Repeat([]byte{0xFF}, 64))
	out := marshalSigned(tok)
	if !bytes.Equal(out[len(out)-64:], bytes.Repeat([]byte{0xFF}, 64)) {
		t.Fatal("expected appended signature")
	}
}
