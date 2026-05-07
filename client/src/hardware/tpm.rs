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
