// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/demo-app/main.go — End-to-end demo app exercising create/activate/validate flows.

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newClient() *client {
	return &client{
		baseURL: getenv("LICENSIFY_BASE_URL", "http://localhost:8080"),
		apiKey:  getenv("LICENSIFY_API_KEY", "dev"),
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *client) do(ctx context.Context, method, path string, body any, out any, auth bool) (*http.Response, []byte, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	if resp.StatusCode >= 400 {
		return resp, data, fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp, data, err
		}
	}
	return resp, data, nil
}

func main() {
	ctx := context.Background()
	c := newClient()

	// 1) Health (public)
	if _, _, err := c.do(ctx, http.MethodGet, "/v1/health", nil, nil, false); err != nil {
		panic(err)
	}
	fmt.Println("health: ok")

	// 2) Create license (admin)
	var created struct {
		ID string `json:"id"`
	}
	createReq := map[string]any{
		"license_key":  "LICENSE-KEY-DEV",
		"license_type": "perpetual",
		"entitlements": map[string]any{
			"license_type":        "perpetual",
			"features":            []string{"base", "pro"},
			"offline_grace_days":  7,
			"custom_metadata":     map[string]any{"env": "demo"},
			"max_activations":     nil,
			"seat_count":          nil,
			"trial_ends_at":       nil,
			"subscription_expires_at": nil,
		},
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/v1/license", createReq, &created, true); err != nil {
		panic(err)
	}
	fmt.Println("license: created", created.ID)

	// 3) Activate: store opaque record
	machine := make([]byte, 32)
	opaqueUpload := make([]byte, 32)
	_, _ = rand.Read(machine)
	_, _ = rand.Read(opaqueUpload)
	actReq := map[string]any{
		"license_id": created.ID,
		"machine_id": base64.StdEncoding.EncodeToString(machine),
		"opaque_registration_upload": base64.StdEncoding.EncodeToString(opaqueUpload),
		"hardware_components": map[string]string{
			"cpuid_brand": "demo",
			"mac_addr":    "00:11:22:33:44:55",
		},
	}
	var actResp map[string]any
	if _, _, err := c.do(ctx, http.MethodPost, "/v1/activate", actReq, &actResp, true); err != nil {
		panic(err)
	}
	fmt.Println("activate: ok (certs issued)")

	// 4) Validate: get a token
	loginReq := make([]byte, 32)
	_, _ = rand.Read(loginReq)
	valReq := map[string]any{
		"license_id":           created.ID,
		"machine_id":           base64.StdEncoding.EncodeToString(machine),
		"opaque_login_request": base64.StdEncoding.EncodeToString(loginReq),
		"client_nonce":         base64.StdEncoding.EncodeToString([]byte("demo")),
	}
	var valResp struct {
		LicenseToken []byte `json:"license_token"`
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/v1/validate", valReq, &valResp, true); err != nil {
		panic(err)
	}
	if len(valResp.LicenseToken) == 0 {
		panic("validate returned empty token")
	}
	fmt.Println("validate: ok (token bytes:", len(valResp.LicenseToken), ")")

	fmt.Println("demo: success")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

