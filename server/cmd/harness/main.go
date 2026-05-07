// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/cmd/harness/main.go — Standalone harness that boots an in-memory Licensify server and seeds a license, used by every SDK e2e test.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rithulkamesh/licensify/server/internal/api"
	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
	"github.com/rithulkamesh/licensify/server/internal/license"
)

// Descriptor is the contract every SDK e2e test consumes.
// The path is configured via LICENSIFY_HARNESS_DESCRIPTOR (default /tmp/licensify-harness.json).
type Descriptor struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	LicenseKey string `json:"license_key"`
	LicenseID  string `json:"license_id"`
}

func main() {
	apiKey := getenv("LICENSIFY_HARNESS_API_KEY", "harness-key")
	licenseKey := getenv("LICENSIFY_HARNESS_LICENSE_KEY", "LICENSE-KEY-DEV")
	descriptorPath := getenv("LICENSIFY_HARNESS_DESCRIPTOR", "/tmp/licensify-harness.json")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	store := db.NewMemStore()
	authority, err := ca.NewAuthority()
	if err != nil {
		fail("ca: %v", err)
	}
	srv := api.New(store, authority, apiKey)

	// Seed a license up front so SDKs can exercise activate/check immediately.
	seeded, err := license.NewService(store).Create(
		context.Background(), licenseKey, license.Perpetual,
		license.Entitlements{
			LicenseType:      license.Perpetual,
			Features:         []string{"base", "pro"},
			OfflineGraceDays: 7,
		}, nil,
	)
	if err != nil {
		fail("seed license: %v", err)
	}

	desc := Descriptor{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		LicenseKey: licenseKey,
		LicenseID:  seeded.ID,
	}
	if err := writeDescriptor(descriptorPath, desc); err != nil {
		fail("write descriptor: %v", err)
	}

	server := &http.Server{Handler: srv.Echo, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		_ = server.Serve(listener)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	fmt.Fprintf(os.Stdout, "harness listening on %s license_id=%s\n", baseURL, seeded.ID)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = os.Remove(descriptorPath)
}

func writeDescriptor(path string, desc Descriptor) error {
	b, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
