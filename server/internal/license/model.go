// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/license/model.go — License domain types used by the server layer.

package license

import (
	"encoding/json"
	"time"
)

type LicenseType string

const (
	Perpetual      LicenseType = "perpetual"
	Subscription   LicenseType = "subscription"
	FloatingSeat   LicenseType = "floating_seat"
	Trial          LicenseType = "trial"
	FeatureFlagged LicenseType = "feature_flagged"
)

type Entitlements struct {
	LicenseType           LicenseType             `json:"license_type"`
	Features              []string                `json:"features"`
	SeatCount             *uint32                 `json:"seat_count,omitempty"`
	MaxActivations        *uint32                 `json:"max_activations,omitempty"`
	TrialEndsAt           *int64                  `json:"trial_ends_at,omitempty"`
	SubscriptionExpiresAt *int64                  `json:"subscription_expires_at,omitempty"`
	OfflineGraceDays      uint32                  `json:"offline_grace_days"`
	CustomMetadata        map[string]any          `json:"custom_metadata"`
}

type License struct {
	ID             string          `json:"id"`
	KeyHash        []byte          `json:"-"`
	OpaqueRecord   []byte          `json:"-"`
	LicenseType    LicenseType     `json:"license_type"`
	Entitlements   Entitlements    `json:"entitlements"`
	MaxActivations *int32          `json:"max_activations,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      *time.Time      `json:"expires_at,omitempty"`
	Revoked        bool            `json:"revoked"`
}

func (e Entitlements) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}
