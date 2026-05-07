-- SPDX-License-Identifier: MIT
-- Copyright (c) 2026 Rithul Kamesh
-- Author: Rithul Kamesh <hi@rithul.dev>
-- Description: server/migrations/000001_init.sql — Initial Postgres schema for Licensify server.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS licenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash BYTEA NOT NULL UNIQUE,
    opaque_record BYTEA NOT NULL DEFAULT ''::bytea,
    license_type TEXT NOT NULL,
    entitlements JSONB NOT NULL,
    max_activations INT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS activations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    license_id UUID REFERENCES licenses(id),
    machine_id BYTEA NOT NULL,
    hw_components JSONB NOT NULL,
    leaf_cert BYTEA NOT NULL,
    activated_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ,
    deactivated BOOLEAN DEFAULT FALSE,
    UNIQUE (license_id, machine_id)
);

CREATE TABLE IF NOT EXISTS floating_seats (
    license_id UUID REFERENCES licenses(id),
    machine_id BYTEA NOT NULL,
    acquired_at TIMESTAMPTZ DEFAULT NOW(),
    heartbeat_at TIMESTAMPTZ,
    PRIMARY KEY (license_id, machine_id)
);

CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    license_id UUID REFERENCES licenses(id),
    machine_id BYTEA,
    event_type TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
