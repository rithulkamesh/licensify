// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/ca/ca_test.go — Tests for the certificate authority.

package ca

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/rithulkamesh/licensify/server/internal/license"
)

func TestNewAuthorityProducesValidChain(t *testing.T) {
	a, err := NewAuthority()
	if err != nil {
		t.Fatal(err)
	}
	if len(a.RootCertDER) == 0 || len(a.IntermediateCertDER) == 0 {
		t.Fatal("expected DER bytes for both root and intermediate")
	}
	root, err := x509.ParseCertificate(a.RootCertDER)
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsCA || root.Subject.CommonName != "Licensify Root CA" {
		t.Fatal("root not a CA or has wrong CN")
	}
	intCert, err := x509.ParseCertificate(a.IntermediateCertDER)
	if err != nil {
		t.Fatal(err)
	}
	if intCert.Issuer.CommonName != root.Subject.CommonName {
		t.Fatal("intermediate not signed by root")
	}
}

func TestIssueLeafEmbedsExtensions(t *testing.T) {
	a, err := NewAuthority()
	if err != nil {
		t.Fatal(err)
	}
	seat := uint32(5)
	ent := license.Entitlements{
		LicenseType:      license.Subscription,
		Features:         []string{"base", "pro"},
		SeatCount:        &seat,
		OfflineGraceDays: 9,
	}
	leafDER, key, err := a.IssueLeaf("license-id", "deadbeef", ent, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(leafDER) == 0 || len(key) == 0 {
		t.Fatal("expected non-empty leaf and key")
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "license-id:deadbeef" {
		t.Fatalf("unexpected CN: %s", leaf.Subject.CommonName)
	}
	foundType, foundEnt, foundSeat, foundGrace := false, false, false, false
	for _, ext := range leaf.Extensions {
		switch {
		case ext.Id.Equal(OIDLicenseType):
			foundType = string(ext.Value) == string(license.Subscription)
		case ext.Id.Equal(OIDEntitlements):
			foundEnt = len(ext.Value) > 0
		case ext.Id.Equal(OIDSeatCount):
			foundSeat = len(ext.Value) == 1 && ext.Value[0] == byte(seat)
		case ext.Id.Equal(OIDOfflineGraceDay):
			foundGrace = len(ext.Value) == 1 && ext.Value[0] == 9
		}
	}
	if !(foundType && foundEnt && foundSeat && foundGrace) {
		t.Fatalf("missing extensions: type=%v ent=%v seat=%v grace=%v", foundType, foundEnt, foundSeat, foundGrace)
	}
}

func TestIssueLeafWithoutSeatCount(t *testing.T) {
	a, _ := NewAuthority()
	ent := license.Entitlements{LicenseType: license.Perpetual, OfflineGraceDays: 1}
	leafDER, _, err := a.IssueLeaf("id", "00", ent, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(leafDER)
	for _, ext := range leaf.Extensions {
		if ext.Id.Equal(OIDSeatCount) {
			if len(ext.Value) != 0 {
				t.Fatalf("expected empty seat count extension, got %d bytes", len(ext.Value))
			}
		}
	}
}
