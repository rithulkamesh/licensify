// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/cache.rs — Local token cache implementation for offline validation.

use aes_gcm::aead::{Aead, KeyInit};
use aes_gcm::{Aes256Gcm, Nonce};
use hkdf::Hkdf;
use sha2::Sha256;

use crate::error::LicenseError;
use crate::hardware::machine_id_bytes;

pub fn encrypt_token(token: &[u8]) -> Result<Vec<u8>, LicenseError> {
    let mid = machine_id_bytes();
    let hk = Hkdf::<Sha256>::new(None, &mid);
    let mut key = [0_u8; 32];
    hk.expand(b"licensify-cache-key", &mut key)
        .map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let cipher = Aes256Gcm::new_from_slice(&key).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let nonce = Nonce::from_slice(&mid[..12]);
    let mut out = nonce.to_vec();
    let encrypted = cipher.encrypt(nonce, token).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    out.extend_from_slice(&encrypted);
    Ok(out)
}

pub fn decrypt_token(data: &[u8]) -> Result<Vec<u8>, LicenseError> {
    if data.len() < 12 {
        return Err(LicenseError::InvalidToken);
    }
    let mid = machine_id_bytes();
    let hk = Hkdf::<Sha256>::new(None, &mid);
    let mut key = [0_u8; 32];
    hk.expand(b"licensify-cache-key", &mut key)
        .map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let cipher = Aes256Gcm::new_from_slice(&key).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let nonce = Nonce::from_slice(&data[..12]);
    cipher.decrypt(nonce, &data[12..]).map_err(|e| LicenseError::Crypto(e.to_string()))
}
