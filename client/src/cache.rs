// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/cache.rs — Local token cache implementation for offline validation.

use aes_gcm::aead::{Aead, KeyInit};
use aes_gcm::{Aes256Gcm, Nonce};
use hkdf::Hkdf;
use rand::RngCore;
use sha2::Sha256;

use crate::error::LicenseError;
use crate::hardware::machine_id_bytes;

/// Cache blob format version. Byte 0 of every stored blob. Bumped from the
/// implicit v1 (which reused the machine id as a fixed GCM nonce — a critical
/// nonce-reuse bug) to v2, which stores a fresh random 96-bit nonce per write.
const CACHE_FORMAT_V2: u8 = 2;

/// HKDF salt. Domain-separates the cache key from any other use of the machine
/// fingerprint and pins it to this format revision.
const CACHE_HKDF_SALT: &[u8] = b"licensify-cache-kdf-v2";
const CACHE_HKDF_INFO: &[u8] = b"licensify-cache-key";
const NONCE_LEN: usize = 12;

fn cache_key() -> Result<[u8; 32], LicenseError> {
    let mid = machine_id_bytes();
    let hk = Hkdf::<Sha256>::new(Some(CACHE_HKDF_SALT), &mid);
    let mut key = [0_u8; 32];
    hk.expand(CACHE_HKDF_INFO, &mut key)
        .map_err(|e| LicenseError::Crypto(e.to_string()))?;
    Ok(key)
}

/// Encrypts `token` with a key derived from this machine's hardware fingerprint
/// and a freshly generated random nonce. Layout: `[version | nonce(12) | ct+tag]`.
///
/// The random nonce is the important property here: AES-GCM is catastrophically
/// broken under nonce reuse, and the previous implementation used a fixed,
/// machine-derived nonce for every write.
pub fn encrypt_token(token: &[u8]) -> Result<Vec<u8>, LicenseError> {
    let key = cache_key()?;
    let cipher = Aes256Gcm::new_from_slice(&key).map_err(|e| LicenseError::Crypto(e.to_string()))?;

    let mut nonce_bytes = [0_u8; NONCE_LEN];
    rand::thread_rng().fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    let encrypted = cipher.encrypt(nonce, token).map_err(|e| LicenseError::Crypto(e.to_string()))?;

    let mut out = Vec::with_capacity(1 + NONCE_LEN + encrypted.len());
    out.push(CACHE_FORMAT_V2);
    out.extend_from_slice(&nonce_bytes);
    out.extend_from_slice(&encrypted);
    Ok(out)
}

/// Decrypts a blob produced by [`encrypt_token`] on this same machine. Fails with
/// [`LicenseError::Crypto`] if the machine fingerprint differs (wrong key ⇒ GCM
/// tag mismatch) or the blob was tampered with, and [`LicenseError::InvalidToken`]
/// if the framing is not recognised.
pub fn decrypt_token(data: &[u8]) -> Result<Vec<u8>, LicenseError> {
    if data.len() < 1 + NONCE_LEN {
        return Err(LicenseError::InvalidToken);
    }
    if data[0] != CACHE_FORMAT_V2 {
        return Err(LicenseError::InvalidToken);
    }
    let key = cache_key()?;
    let cipher = Aes256Gcm::new_from_slice(&key).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let nonce = Nonce::from_slice(&data[1..1 + NONCE_LEN]);
    cipher
        .decrypt(nonce, &data[1 + NONCE_LEN..])
        .map_err(|e| LicenseError::Crypto(e.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;

    fn clear_hw_env() {
        for k in [
            "LICENSIFY_MACHINE_ID",
            "LICENSIFY_DISK_SERIAL",
            "LICENSIFY_BOARD_UUID",
            "LICENSIFY_MAC",
            "LICENSIFY_GPU_ID",
            "LICENSIFY_TPM_EK_SEED",
        ] {
            std::env::remove_var(k);
        }
    }

    fn with_machine_id<F: FnOnce()>(id: &str, f: F) {
        clear_hw_env();
        std::env::set_var("LICENSIFY_MACHINE_ID", id);
        f();
        clear_hw_env();
    }

    #[test]
    #[serial]
    fn roundtrip_succeeds() {
        with_machine_id("cache-roundtrip-machine", || {
            let token = b"secret-cache-token-payload";
            let enc = encrypt_token(token).unwrap();
            assert_eq!(enc[0], CACHE_FORMAT_V2);
            let dec = decrypt_token(&enc).unwrap();
            assert_eq!(dec, token);
        });
    }

    #[test]
    #[serial]
    fn each_encryption_uses_a_fresh_nonce() {
        with_machine_id("cache-nonce-machine", || {
            let a = encrypt_token(b"same-plaintext").unwrap();
            let b = encrypt_token(b"same-plaintext").unwrap();
            // Different random nonce ⇒ different framing bytes and ciphertext.
            assert_ne!(a[1..1 + NONCE_LEN], b[1..1 + NONCE_LEN]);
            assert_ne!(a, b);
            // Both still decrypt back to the same plaintext.
            assert_eq!(decrypt_token(&a).unwrap(), b"same-plaintext");
            assert_eq!(decrypt_token(&b).unwrap(), b"same-plaintext");
        });
    }

    #[test]
    #[serial]
    fn decrypt_rejects_short_buffer() {
        let err = decrypt_token(&[0u8; 4]).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidToken));
    }

    #[test]
    #[serial]
    fn decrypt_rejects_unknown_format_byte() {
        let mut enc: Vec<u8> = Vec::new();
        with_machine_id("cache-fmt-machine", || {
            enc = encrypt_token(b"data").unwrap();
        });
        enc[0] = 0x01; // legacy / unknown framing
        with_machine_id("cache-fmt-machine", || {
            assert!(matches!(decrypt_token(&enc).unwrap_err(), LicenseError::InvalidToken));
        });
    }

    #[test]
    #[serial]
    fn decrypt_rejects_machine_mismatch() {
        let mut enc: Vec<u8> = Vec::new();
        with_machine_id("cache-machine-a", || {
            enc = encrypt_token(b"data").unwrap();
        });
        with_machine_id("cache-machine-b", || {
            let err = decrypt_token(&enc).unwrap_err();
            assert!(matches!(err, LicenseError::Crypto(_)));
        });
    }

    #[test]
    #[serial]
    fn decrypt_rejects_tampered_ciphertext() {
        with_machine_id("cache-tamper-machine", || {
            let mut enc = encrypt_token(b"payload").unwrap();
            let last = enc.len() - 1;
            enc[last] ^= 0xFF;
            assert!(matches!(decrypt_token(&enc).unwrap_err(), LicenseError::Crypto(_)));
        });
    }
}
