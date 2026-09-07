// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/api/server.go — HTTP API server and route wiring.

package api

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rithulkamesh/licensify/server/internal/auth"
	"github.com/rithulkamesh/licensify/server/internal/ca"
	"github.com/rithulkamesh/licensify/server/internal/db"
	"github.com/rithulkamesh/licensify/server/internal/license"
	"github.com/rithulkamesh/licensify/server/internal/token"
)

type Server struct {
	Echo     *echo.Echo
	store    db.Store
	license  *license.Service
	auth     *auth.Service
	ca       *ca.Authority
	apiKey   string
	tokenKey ed25519.PrivateKey
	seatMu   sync.Mutex
	seats    map[string]map[string]time.Time
}

// New wires the HTTP API. `apiKey` guards the admin surface (license CRUD,
// seats). `tokenKey` is the stable Ed25519 key that signs offline license
// tokens; its public half is published at `/v1/.well-known/token-key` and is
// what clients verify cached tokens against.
func New(store db.Store, authority *ca.Authority, apiKey string, tokenKey ed25519.PrivateKey) *Server {
	e := echo.New()
	s := &Server{
		Echo:     e,
		store:    store,
		license:  license.NewService(store),
		auth:     auth.NewService(store),
		ca:       authority,
		apiKey:   apiKey,
		tokenKey: tokenKey,
		seats:    map[string]map[string]time.Time{},
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Echo.GET("/v1/health", s.health)
	s.Echo.GET("/v1/.well-known/ca", s.caRoot)
	s.Echo.GET("/v1/.well-known/token-key", s.tokenKeyPub)

	// Admin surface: guarded by the API key.
	admin := s.Echo.Group("/v1")
	admin.Use(s.requireAPIKey)
	admin.POST("/license", s.createLicense)
	admin.GET("/license/:id", s.getLicense)
	admin.PUT("/license/:id", s.updateLicense)
	admin.GET("/seats/:id", s.seatStatus)
	admin.POST("/seats/:id/acquire", s.seatAcquire)
	admin.POST("/seats/:id/release", s.seatRelease)

	// Client surface: authenticated by license-key possession, not the admin
	// API key. A client SDK should never need to ship the admin key.
	client := s.Echo.Group("/v1")
	client.POST("/activate", s.activate)
	client.POST("/validate", s.validate)
	client.POST("/deactivate", s.deactivate)
	client.POST("/heartbeat", s.heartbeat)
}

func (s *Server) tokenKeyPub(c echo.Context) error {
	return c.String(http.StatusOK, token.PublicKeyHex(s.tokenKey))
}

func (s *Server) requireAPIKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if s.apiKey == "" || c.Request().Header.Get("X-API-Key") == s.apiKey {
			return next(c)
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
}

func (s *Server) health(c echo.Context) error {
	if err := s.store.Health(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "down", "error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok", "version": "0.1.0"})
}

func (s *Server) caRoot(c echo.Context) error {
	return c.Blob(http.StatusOK, "application/octet-stream", s.ca.RootCertDER)
}

func (s *Server) createLicense(c echo.Context) error {
	var req struct {
		LicenseKey   string               `json:"license_key"`
		LicenseType  license.LicenseType  `json:"license_type"`
		Entitlements license.Entitlements `json:"entitlements"`
		ExpiresAt    *int64               `json:"expires_at"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var exp *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		exp = &t
	}
	l, err := s.license.Create(c.Request().Context(), req.LicenseKey, req.LicenseType, req.Entitlements, exp)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, l)
}

func (s *Server) getLicense(c echo.Context) error {
	l, err := s.license.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, l)
}

func (s *Server) updateLicense(c echo.Context) error {
	l, err := s.license.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	if err := c.Bind(&l); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	l.ID = c.Param("id")
	l, err = s.license.Update(c.Request().Context(), l)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, l)
}

// resolveClientLicense authenticates a client request by license-key possession
// and returns the matching license.
func (s *Server) resolveClientLicense(c echo.Context, key string) (license.License, bool) {
	if key == "" {
		_ = c.JSON(http.StatusUnauthorized, map[string]string{"error": "license_key required"})
		return license.License{}, false
	}
	l, err := s.license.GetByKey(c.Request().Context(), key)
	if err != nil {
		_ = c.JSON(http.StatusUnauthorized, map[string]string{"error": "unknown license_key"})
		return license.License{}, false
	}
	return l, true
}

func (s *Server) activate(c echo.Context) error {
	var req struct {
		LicenseKey   string            `json:"license_key"`
		MachineID    []byte            `json:"machine_id"`
		OpaqueUpload []byte            `json:"opaque_registration_upload"`
		HWComponents map[string]string `json:"hardware_components"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	l, ok := s.resolveClientLicense(c, req.LicenseKey)
	if !ok {
		return nil
	}
	if err := s.auth.Register(c.Request().Context(), l.ID, req.OpaqueUpload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	leaf, _, err := s.ca.IssueLeaf(l.ID, license.StableMachineID(req.MachineID), l.Entitlements, time.Now().UTC().AddDate(1, 0, 0))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"leaf_certificate": leaf, "intermediate_certificate": s.ca.IntermediateCertDER})
}

func (s *Server) validate(c echo.Context) error {
	var req struct {
		LicenseKey  string `json:"license_key"`
		MachineID   []byte `json:"machine_id"`
		OpaqueReq   []byte `json:"opaque_login_request"`
		ClientNonce []byte `json:"client_nonce"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	l, ok := s.resolveClientLicense(c, req.LicenseKey)
	if !ok {
		return nil
	}
	if _, _, err := s.auth.StartLogin(c.Request().Context(), l.ID, req.OpaqueReq); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	entJSON, _ := json.Marshal(l.Entitlements)
	// Bind the token to a stable hash of the client's machine id, and sign it
	// with the server's stable token key so the client can verify it offline
	// against the key published at /v1/.well-known/token-key.
	hash := sha256.Sum256(req.MachineID)
	tok, err := token.Build(l.ID, hash, entJSON, 30*24*time.Hour, s.tokenKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"license_token": tok})
}

func (s *Server) deactivate(c echo.Context) error {
	var req struct {
		LicenseKey string `json:"license_key"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if _, ok := s.resolveClientLicense(c, req.LicenseKey); !ok {
		return nil
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) seatStatus(c echo.Context) error {
	id := c.Param("id")
	s.seatMu.Lock()
	defer s.seatMu.Unlock()
	m := s.seats[id]
	out := make([]string, 0, len(m))
	for machine := range m {
		out = append(out, machine)
	}
	return c.JSON(http.StatusOK, map[string]any{"license_id": id, "active_seats": len(out), "machine_ids": out})
}

func (s *Server) seatAcquire(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		MachineID string `json:"machine_id"`
	}
	if err := c.Bind(&req); err != nil || req.MachineID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "machine_id required"})
	}
	s.seatMu.Lock()
	defer s.seatMu.Unlock()
	if _, ok := s.seats[id]; !ok {
		s.seats[id] = map[string]time.Time{}
	}
	s.seats[id][req.MachineID] = time.Now().UTC()
	return c.JSON(http.StatusOK, map[string]any{"success": true, "active_seats": len(s.seats[id])})
}

func (s *Server) seatRelease(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		MachineID string `json:"machine_id"`
	}
	if err := c.Bind(&req); err != nil || req.MachineID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "machine_id required"})
	}
	s.seatMu.Lock()
	defer s.seatMu.Unlock()
	if _, ok := s.seats[id]; ok {
		delete(s.seats[id], req.MachineID)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "active_seats": len(s.seats[id])})
}

func (s *Server) heartbeat(c echo.Context) error {
	var req struct {
		LicenseKey string `json:"license_key"`
		MachineID  string `json:"machine_id"`
		Nonce      string `json:"nonce"`
	}
	if err := c.Bind(&req); err != nil || req.MachineID == "" || req.Nonce == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "license_key, machine_id and nonce required"})
	}
	if _, ok := s.resolveClientLicense(c, req.LicenseKey); !ok {
		return nil
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":         true,
		"receivedAt": time.Now().UTC().Unix(),
	})
}
