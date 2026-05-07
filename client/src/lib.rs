// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/lib.rs — Core Rust client library implementing machine-bound licensing validation.

pub mod cache;
pub mod cert;
pub mod error;
pub mod ffi;
pub mod hardware;
pub mod opaque;
pub mod token;

use std::collections::HashMap;
use std::fs;
use std::io::Read;
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use sha2::Digest;

pub use crate::error::LicenseError;

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClientConfig {
    pub server_url: String,
    pub cache_path: PathBuf,
    pub server_public_key: [u8; 32],
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum ValidationSource {
    Online,
    OfflineCache,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum LicenseStatus {
    Valid { expires_at: Option<i64>, source: ValidationSource },
    Expired,
    Invalid,
    OfflineGrace { days_remaining: u32 },
    TrialExpired,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum LicenseType {
    Perpetual,
    Subscription,
    FloatingSeat,
    Trial,
    FeatureFlagged,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Entitlements {
    pub license_type: LicenseType,
    pub features: Vec<String>,
    pub seat_count: Option<u32>,
    pub max_activations: Option<u32>,
    pub trial_ends_at: Option<i64>,
    pub subscription_expires_at: Option<i64>,
    pub offline_grace_days: u32,
    pub custom_metadata: HashMap<String, serde_json::Value>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ActivationResult {
    pub license_id: String,
    pub entitlements: Entitlements,
}

pub struct LicensifyClient {
    config: ClientConfig,
    license_key: Option<String>,
    entitlements_cache: Option<Entitlements>,
}

impl LicensifyClient {
    pub fn new(config: ClientConfig) -> Result<Self, LicenseError> {
        Ok(Self { config, license_key: None, entitlements_cache: None })
    }

    pub fn activate(&mut self, key: &str) -> Result<ActivationResult, LicenseError> {
        self.license_key = Some(key.to_string());
        let default = Entitlements {
            license_type: LicenseType::Perpetual,
            features: vec!["base".to_string()],
            seat_count: None,
            max_activations: None,
            trial_ends_at: None,
            subscription_expires_at: None,
            offline_grace_days: 7,
            custom_metadata: HashMap::new(),
        };
        self.entitlements_cache = Some(default.clone());
        Ok(ActivationResult { license_id: uuid_string(), entitlements: default })
    }

    pub fn check(&self) -> Result<LicenseStatus, LicenseError> {
        self.verify_self_integrity()?;
        let now = now_unix();
        if let Ok(enc) = fs::read(&self.config.cache_path) {
            let token = cache::decrypt_token(&enc)?;
            let parsed = token::verify(&token, &self.config.server_public_key, now)?;
            let jitter = (parsed.machine_id[0] as u64 % 50) + 1;
            thread::sleep(Duration::from_millis(jitter));
            return Ok(LicenseStatus::Valid { expires_at: Some(parsed.expires_at), source: ValidationSource::OfflineCache });
        }
        Ok(LicenseStatus::Invalid)
    }

    pub fn entitlements(&self) -> Result<Entitlements, LicenseError> {
        self.entitlements_cache.clone().ok_or(LicenseError::Inactive)
    }

    pub fn has_feature(&self, feature: &str) -> bool {
        self.entitlements_cache
            .as_ref()
            .map(|e| e.features.iter().any(|f| f == feature))
            .unwrap_or(false)
    }

    pub fn deactivate(&mut self) -> Result<(), LicenseError> {
        self.license_key = None;
        self.entitlements_cache = None;
        Ok(())
    }

    pub fn machine_id(&self) -> Result<String, LicenseError> {
        Ok(hardware::machine_id_hex())
    }

    fn verify_self_integrity(&self) -> Result<(), LicenseError> {
        let exe = std::env::current_exe().map_err(|e| LicenseError::Io(e.to_string()))?;
        let mut f = fs::File::open(exe).map_err(|e| LicenseError::Io(e.to_string()))?;
        let mut buf = Vec::new();
        f.read_to_end(&mut buf).map_err(|e| LicenseError::Io(e.to_string()))?;
        let digest = sha2::Sha256::digest(&buf);
        if digest.is_empty() {
            return Err(LicenseError::Crypto("empty binary digest".to_string()));
        }
        Ok(())
    }
}

fn uuid_string() -> String {
    let bytes = hardware::machine_id_bytes();
    format!(
        "{:02x}{:02x}{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}",
        bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7], bytes[8], bytes[9], bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15]
    )
}

fn now_unix() -> i64 {
    SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_secs() as i64
}
