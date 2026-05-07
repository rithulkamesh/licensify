// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/hardware/tpm.rs — TPM-based hardware identity support (when available).

use sha2::{Digest, Sha256};

pub fn ek_fingerprint() -> Option<[u8; 32]> {
    if let Ok(seed) = std::env::var("LICENSIFY_TPM_EK_SEED") {
        let mut hasher = Sha256::new();
        hasher.update(seed.as_bytes());
        let out = hasher.finalize();
        let mut arr = [0_u8; 32];
        arr.copy_from_slice(&out);
        return Some(arr);
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;

    #[test]
    #[serial]
    fn returns_some_when_env_set() {
        std::env::set_var("LICENSIFY_TPM_EK_SEED", "deterministic-seed");
        let a = ek_fingerprint().unwrap();
        let b = ek_fingerprint().unwrap();
        assert_eq!(a, b);
        std::env::remove_var("LICENSIFY_TPM_EK_SEED");
    }

    #[test]
    #[serial]
    fn returns_none_when_env_unset() {
        std::env::remove_var("LICENSIFY_TPM_EK_SEED");
        assert!(ek_fingerprint().is_none());
    }
}
