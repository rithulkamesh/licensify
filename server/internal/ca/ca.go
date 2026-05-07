// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/ca/ca.go — Certificate authority creation and certificate issuance utilities.

package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	"github.com/rithulkamesh/licensify/server/internal/license"
)

var (
	OIDLicenseType     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1}
	OIDEntitlements    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 2}
	OIDSeatCount       = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 3}
	OIDOfflineGraceDay = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 4}
)

type Authority struct {
	RootCertDER         []byte
	IntermediateCertDER []byte
	rootCert            *x509.Certificate
	rootKey             ed25519.PrivateKey
	intCert             *x509.Certificate
	intKey              ed25519.PrivateKey
}

func NewAuthority() (*Authority, error) {
	rootPub, rootKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rootTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Licensify Root CA",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(20, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTpl, rootTpl, rootPub, rootKey)
	if err != nil {
		return nil, err
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return nil, err
	}

	intPub, intKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	intTpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Licensify Intermediate CA",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
	}
	intDER, err := x509.CreateCertificate(rand.Reader, intTpl, rootCert, intPub, rootKey)
	if err != nil {
		return nil, err
	}
	intCert, err := x509.ParseCertificate(intDER)
	if err != nil {
		return nil, err
	}

	return &Authority{
		RootCertDER:         rootDER,
		IntermediateCertDER: intDER,
		rootCert:            rootCert,
		rootKey:             rootKey,
		intCert:             intCert,
		intKey:              intKey,
	}, nil
}

func (a *Authority) IssueLeaf(licenseID string, machineHash string, ent license.Entitlements, notAfter time.Time) ([]byte, ed25519.PrivateKey, error) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	entJSON, err := ent.ToJSON()
	if err != nil {
		return nil, nil, err
	}
	seatCount := []byte{}
	if ent.SeatCount != nil {
		seatCount = []byte{byte(*ent.SeatCount)}
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: licenseID + ":" + machineHash,
		},
		NotBefore: time.Now().UTC().Add(-time.Minute),
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			{Id: OIDLicenseType, Value: []byte(ent.LicenseType)},
			{Id: OIDEntitlements, Value: entJSON},
			{Id: OIDSeatCount, Value: seatCount},
			{Id: OIDOfflineGraceDay, Value: []byte{byte(ent.OfflineGraceDays)}},
		},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, a.intCert, pub, a.intKey)
	if err != nil {
		return nil, nil, err
	}
	return der, key, nil
}
