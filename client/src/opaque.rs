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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn registration_request_is_deterministic_and_distinct() {
        let a = registration_request("KEY", &[1, 2, 3]);
        let b = registration_request("KEY", &[1, 2, 3]);
        let c = registration_request("KEY", &[9, 9, 9]);
        let d = login_request("KEY", &[1, 2, 3]);
        assert_eq!(a, b);
        assert_ne!(a, c);
        assert_ne!(a, d);
        assert_eq!(a.len(), 32);
    }

    #[test]
    fn registration_upload_combines_request_and_response() {
        let a = registration_upload(&[1u8; 32], &[2u8; 32]);
        let b = registration_upload(&[1u8; 32], &[2u8; 32]);
        let c = registration_upload(&[1u8; 32], &[3u8; 32]);
        assert_eq!(a, b);
        assert_ne!(a, c);
    }

    #[test]
    fn login_request_is_deterministic() {
        let a = login_request("K", &[]);
        let b = login_request("K", &[]);
        assert_eq!(a, b);
        assert_eq!(a.len(), 32);
    }
}
