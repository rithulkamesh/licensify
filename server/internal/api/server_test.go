// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/api/server_test.go — Tests covering every HTTP handler with happy and error paths.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
	"github.com/rithulkamesh/licensify/server/internal/license"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	authority, err := ca.NewAuthority()
	if err != nil {
		t.Fatal(err)
	}
	return New(db.NewMemStore(), authority, "secret")
}

func do(t *testing.T, s *Server, method, path string, body any, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec, req)
	return rec
}

func doRaw(t *testing.T, s *Server, method, path, body, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	s.Echo.ServeHTTP(rec, req)
	return rec
}

func createLicense(t *testing.T, s *Server) string {
	t.Helper()
	body := map[string]any{
		"license_key":  "LICENSE-KEY-DEV",
		"license_type": "perpetual",
		"entitlements": map[string]any{
			"license_type":       "perpetual",
			"features":           []string{"base", "pro"},
			"offline_grace_days": 7,
		},
	}
	rec := do(t, s, http.MethodPost, "/v1/license", body, "secret")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var l license.License
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	return l.ID
}

// --- public routes ---

func TestHealthAndCaArePublic(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/v1/health", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("health: %d", rec.Code)
	}
	rec2 := do(t, s, http.MethodGet, "/v1/.well-known/ca", nil, "")
	if rec2.Code != http.StatusOK || rec2.Body.Len() == 0 {
		t.Fatalf("ca: %d", rec2.Code)
	}
}

func TestHealthReturnsDownWhenStoreDown(t *testing.T) {
	auth, _ := ca.NewAuthority()
	s := New(newDownStore(), auth, "secret")
	rec := do(t, s, http.MethodGet, "/v1/health", nil, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

// --- auth ---

func TestProtectedRoutesRequireApiKey(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/v1/license", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}

func TestEmptyApiKeyDisablesAuth(t *testing.T) {
	auth, _ := ca.NewAuthority()
	s := New(db.NewMemStore(), auth, "")
	rec := do(t, s, http.MethodPost, "/v1/license", map[string]any{
		"license_key":  "k",
		"license_type": "perpetual",
		"entitlements": map[string]any{"license_type": "perpetual"},
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rec.Code, rec.Body.String())
	}
}

// --- license CRUD ---

func TestCreateLicenseHappyAndBadBind(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	if id == "" {
		t.Fatal("expected id")
	}
	bad := doRaw(t, s, http.MethodPost, "/v1/license", "{not json", "secret")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.Code)
	}
}

func TestCreateLicenseWithExpiresAt(t *testing.T) {
	s := newTestServer(t)
	exp := int64(1900000000)
	rec := do(t, s, http.MethodPost, "/v1/license", map[string]any{
		"license_key":  "expiring",
		"license_type": "subscription",
		"entitlements": map[string]any{"license_type": "subscription"},
		"expires_at":   exp,
	}, "secret")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateLicenseRejectsEmptyKey(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/v1/license", map[string]any{
		"license_key":  "",
		"license_type": "perpetual",
	}, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetLicense(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	rec := do(t, s, http.MethodGet, "/v1/license/"+id, nil, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	rec2 := do(t, s, http.MethodGet, "/v1/license/missing", nil, "secret")
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec2.Code)
	}
}

func TestUpdateLicense(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	rec := do(t, s, http.MethodPut, "/v1/license/"+id, map[string]any{"revoked": true}, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	miss := do(t, s, http.MethodPut, "/v1/license/missing", map[string]any{}, "secret")
	if miss.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", miss.Code)
	}
	bad := doRaw(t, s, http.MethodPut, "/v1/license/"+id, "not json", "secret")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.Code)
	}
}

// --- activate / validate ---

func TestActivateAndValidate(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	mid := bytes.Repeat([]byte{0xAB}, 32)
	upload := []byte("opaque-upload")
	rec := do(t, s, http.MethodPost, "/v1/activate", map[string]any{
		"license_id":                 id,
		"machine_id":                 mid,
		"opaque_registration_upload": upload,
	}, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("activate: %d %s", rec.Code, rec.Body.String())
	}

	val := do(t, s, http.MethodPost, "/v1/validate", map[string]any{
		"license_id":           id,
		"machine_id":           mid,
		"opaque_login_request": []byte("login"),
		"client_nonce":         []byte("nonce"),
	}, "secret")
	if val.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", val.Code, val.Body.String())
	}
}

func TestActivateBadBind(t *testing.T) {
	s := newTestServer(t)
	rec := doRaw(t, s, http.MethodPost, "/v1/activate", "{not json", "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestActivateRejectsEmptyUpload(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	rec := do(t, s, http.MethodPost, "/v1/activate", map[string]any{
		"license_id": id,
		"machine_id": []byte{1, 2, 3},
	}, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestActivateMissingLicenseAfterRegister(t *testing.T) {
	auth, _ := ca.NewAuthority()
	s := New(&licenseMissingStore{MemStore: db.NewMemStore()}, auth, "secret")
	rec := do(t, s, http.MethodPost, "/v1/activate", map[string]any{
		"license_id":                 "ghost",
		"machine_id":                 []byte{1},
		"opaque_registration_upload": []byte{2},
	}, "secret")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestValidateMissingRecord(t *testing.T) {
	s := newTestServer(t)
	id := createLicense(t, s)
	rec := do(t, s, http.MethodPost, "/v1/validate", map[string]any{
		"license_id":           id,
		"machine_id":           []byte{1},
		"opaque_login_request": []byte("login"),
	}, "secret")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (no opaque record), got %d", rec.Code)
	}
}

func TestValidateBadBind(t *testing.T) {
	s := newTestServer(t)
	rec := doRaw(t, s, http.MethodPost, "/v1/validate", "{not json", "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestValidateMissingLicense(t *testing.T) {
	auth, _ := ca.NewAuthority()
	st := db.NewMemStore()
	_ = st.StoreOpaqueRecord(context.Background(), "ghost", []byte("rec"))
	s := New(st, auth, "secret")
	rec := do(t, s, http.MethodPost, "/v1/validate", map[string]any{
		"license_id":           "ghost",
		"machine_id":           []byte{1},
		"opaque_login_request": []byte("l"),
	}, "secret")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- deactivate / heartbeat ---

func TestDeactivateAlwaysOk(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/v1/deactivate", map[string]any{}, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate: %d", rec.Code)
	}
}

func TestHeartbeatHappyAndBad(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/v1/heartbeat", map[string]any{
		"license_id": "x", "machine_id": "y", "nonce": "z",
	}, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat: %d", rec.Code)
	}
	bad := do(t, s, http.MethodPost, "/v1/heartbeat", map[string]any{}, "secret")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.Code)
	}
	bad2 := doRaw(t, s, http.MethodPost, "/v1/heartbeat", "{not json", "secret")
	if bad2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad2.Code)
	}
}

// --- seats ---

func TestSeatLifecycle(t *testing.T) {
	s := newTestServer(t)
	id := "seat-license"
	st := do(t, s, http.MethodGet, "/v1/seats/"+id, nil, "secret")
	if st.Code != http.StatusOK {
		t.Fatalf("seat status: %d", st.Code)
	}
	acq := do(t, s, http.MethodPost, "/v1/seats/"+id+"/acquire", map[string]any{"machine_id": "m1"}, "secret")
	if acq.Code != http.StatusOK {
		t.Fatalf("seat acquire: %d", acq.Code)
	}
	// Status AFTER acquire exercises the loop body that emits machine IDs.
	stAfter := do(t, s, http.MethodGet, "/v1/seats/"+id, nil, "secret")
	if stAfter.Code != http.StatusOK || !strings.Contains(stAfter.Body.String(), "m1") {
		t.Fatalf("seat status after acquire: %d body=%s", stAfter.Code, stAfter.Body.String())
	}
	acqBad := do(t, s, http.MethodPost, "/v1/seats/"+id+"/acquire", map[string]any{}, "secret")
	if acqBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", acqBad.Code)
	}
	rel := do(t, s, http.MethodPost, "/v1/seats/"+id+"/release", map[string]any{"machine_id": "m1"}, "secret")
	if rel.Code != http.StatusOK {
		t.Fatalf("seat release: %d", rel.Code)
	}
	relMissing := do(t, s, http.MethodPost, "/v1/seats/other-license/release", map[string]any{"machine_id": "m1"}, "secret")
	if relMissing.Code != http.StatusOK {
		t.Fatalf("expected 200 even for missing license, got %d", relMissing.Code)
	}
	relBad := do(t, s, http.MethodPost, "/v1/seats/"+id+"/release", map[string]any{}, "secret")
	if relBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", relBad.Code)
	}
}

// --- helper stores ---

type downStore struct{ *db.MemStore }

func (downStore) Health(_ context.Context) error { return errors.New("down") }

func newDownStore() *downStore { return &downStore{MemStore: db.NewMemStore()} }

// licenseMissingStore makes Activate's auth.Register succeed but license.Get fail.
type licenseMissingStore struct{ *db.MemStore }

func (s *licenseMissingStore) GetByID(_ context.Context, _ string) (license.License, error) {
	return license.License{}, errors.New("license not found")
}

// updateFailStore allows GetByID but rejects Update to exercise the 400 branch.
type updateFailStore struct{ *db.MemStore }

func (s *updateFailStore) Update(_ context.Context, _ license.License) (license.License, error) {
	return license.License{}, errors.New("update rejected")
}

// badIDStore returns a license whose ID is not a valid UUID, forcing
// `token.Build` to fail with an InvalidArgument error in the validate handler.
type badIDStore struct{ *db.MemStore }

func (s *badIDStore) GetByID(_ context.Context, _ string) (license.License, error) {
	return license.License{ID: "not-a-uuid", LicenseType: license.Perpetual}, nil
}

func TestUpdateLicensePropagatesStoreError(t *testing.T) {
	auth, _ := ca.NewAuthority()
	st := db.NewMemStore()
	id := "id-update-err"
	_, _ = st.Create(context.Background(), license.License{ID: id, LicenseType: license.Perpetual})
	s := New(&updateFailStore{MemStore: st}, auth, "secret")
	rec := do(t, s, http.MethodPut, "/v1/license/"+id, map[string]any{"revoked": true}, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestValidateTokenBuildErrorReturns500(t *testing.T) {
	auth, _ := ca.NewAuthority()
	st := db.NewMemStore()
	_ = st.StoreOpaqueRecord(context.Background(), "anything", []byte("rec"))
	s := New(&badIDStore{MemStore: st}, auth, "secret")
	rec := do(t, s, http.MethodPost, "/v1/validate", map[string]any{
		"license_id":           "anything",
		"machine_id":           []byte{1},
		"opaque_login_request": []byte("l"),
	}, "secret")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from token.Build, got %d body=%s", rec.Code, rec.Body.String())
	}
}
