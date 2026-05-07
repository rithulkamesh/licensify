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

	// basic field sanity: version + fixed sizes exist
	if unsigned[0] != 1 {
		t.Fatalf("unexpected version: %d", unsigned[0])
	}
	at := 1 + 16 + 32 + 8 + 8
	entLen := binary.BigEndian.Uint32(unsigned[at : at+4])
	if entLen != 3 {
		t.Fatalf("unexpected ent len: %d", entLen)
	}
}

