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
	Echo    *echo.Echo
	store   db.Store
	license *license.Service
	auth    *auth.Service
	ca      *ca.Authority
	apiKey  string
	seatMu  sync.Mutex
	seats   map[string]map[string]time.Time
}

func New(store db.Store, authority *ca.Authority, apiKey string) *Server {
	e := echo.New()
	s := &Server{
		Echo:    e,
		store:   store,
		license: license.NewService(store),
		auth:    auth.NewService(store),
		ca:      authority,
		apiKey:  apiKey,
		seats:   map[string]map[string]time.Time{},
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Echo.GET("/v1/health", s.health)
	s.Echo.GET("/v1/.well-known/ca", s.caRoot)

	g := s.Echo.Group("/v1")
	g.Use(s.requireAPIKey)
	g.POST("/license", s.createLicense)
	g.GET("/license/:id", s.getLicense)
	g.PUT("/license/:id", s.updateLicense)
	g.POST("/activate", s.activate)
	g.POST("/validate", s.validate)
	g.POST("/deactivate", s.deactivate)
	g.POST("/heartbeat", s.heartbeat)
	g.GET("/seats/:id", s.seatStatus)
	g.POST("/seats/:id/acquire", s.seatAcquire)
	g.POST("/seats/:id/release", s.seatRelease)
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

func (s *Server) activate(c echo.Context) error {
	var req struct {
		LicenseID      string            `json:"license_id"`
		MachineID      []byte            `json:"machine_id"`
		OpaqueUpload   []byte            `json:"opaque_registration_upload"`
		HWComponents   map[string]string `json:"hardware_components"`
		LicenseKeyHash []byte            `json:"license_key_hash"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := s.auth.Register(c.Request().Context(), req.LicenseID, req.OpaqueUpload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	l, err := s.license.Get(c.Request().Context(), req.LicenseID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	leaf, _, err := s.ca.IssueLeaf(l.ID, license.StableMachineID(req.MachineID), l.Entitlements, time.Now().UTC().AddDate(1, 0, 0))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"leaf_certificate": leaf, "intermediate_certificate": s.ca.IntermediateCertDER})
}

func (s *Server) validate(c echo.Context) error {
	var req struct {
		LicenseID  string `json:"license_id"`
		MachineID  []byte `json:"machine_id"`
		OpaqueReq  []byte `json:"opaque_login_request"`
		ClientNonce []byte `json:"client_nonce"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	_, sessionKey, err := s.auth.StartLogin(c.Request().Context(), req.LicenseID, req.OpaqueReq)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	l, err := s.license.Get(c.Request().Context(), req.LicenseID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	entJSON, _ := json.Marshal(l.Entitlements)
	hash := sha256.Sum256(req.MachineID)
	seed := sha256.Sum256(sessionKey)
	signingKey := ed25519.NewKeyFromSeed(seed[:])
	tok, err := token.Build(l.ID, hash, entJSON, 30*24*time.Hour, signingKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"license_token": tok})
}

func (s *Server) deactivate(c echo.Context) error {
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
		LicenseID string `json:"license_id"`
		MachineID string `json:"machine_id"`
		Nonce     string `json:"nonce"`
	}
	if err := c.Bind(&req); err != nil || req.LicenseID == "" || req.MachineID == "" || req.Nonce == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "license_id, machine_id and nonce required"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":         true,
		"receivedAt": time.Now().UTC().Unix(),
	})
}
