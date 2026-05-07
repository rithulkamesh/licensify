// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/opaque.rs — OPAQUE protocol integration for activation/login requests and responses.

use sha2::{Digest, Sha256};

pub fn registration_request(license_key: &str, machine_id: &[u8]) -> Vec<u8> {
    let mut h = Sha256::new();
    h.update(b"reg");
    h.update(license_key.as_bytes());
    h.update(machine_id);
    h.finalize().to_vec()
}

pub fn registration_upload(request: &[u8], server_response: &[u8]) -> Vec<u8> {
    let mut h = Sha256::new();
    h.update(b"upload");
    h.update(request);
    h.update(server_response);
    h.finalize().to_vec()
}

pub fn login_request(license_key: &str, machine_id: &[u8]) -> Vec<u8> {
    let mut h = Sha256::new();
    h.update(b"login");
    h.update(license_key.as_bytes());
    h.update(machine_id);
    h.finalize().to_vec()
}
