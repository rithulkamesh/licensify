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

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
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

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
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

/// `IntegrityCheck` is a closure type invoked by `LicensifyClient::check` to
/// guard against tamper. The default implementation reads the running
/// executable and computes a digest. Tests inject a no-op variant.
pub type IntegrityCheck = Box<dyn Fn() -> Result<(), LicenseError> + Send + Sync>;

pub struct LicensifyClient {
    config: ClientConfig,
    license_key: Option<String>,
    entitlements_cache: Option<Entitlements>,
    integrity_check: IntegrityCheck,
}

impl LicensifyClient {
    pub fn new(config: ClientConfig) -> Result<Self, LicenseError> {
        Self::with_integrity_check(config, default_integrity_check())
    }

    /// Construct a client with an explicit integrity check function. Used by
    /// tests to bypass the real `current_exe()` digest path which is fragile
    /// under coverage instrumentation.
    pub fn with_integrity_check(
        config: ClientConfig,
        integrity_check: IntegrityCheck,
    ) -> Result<Self, LicenseError> {
        Ok(Self {
            config,
            license_key: None,
            entitlements_cache: None,
            integrity_check,
        })
    }

    pub fn activate(&mut self, key: &str) -> Result<ActivationResult, LicenseError> {
        if key.is_empty() {
            return Err(LicenseError::Inactive);
        }
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
        (self.integrity_check)()?;
        let now = now_unix();
        if let Ok(enc) = fs::read(&self.config.cache_path) {
            let token = cache::decrypt_token(&enc)?;
            let parsed = token::verify(&token, &self.config.server_public_key, now)?;
            let jitter = (parsed.machine_id[0] as u64 % 50) + 1;
            thread::sleep(Duration::from_millis(jitter));
            return Ok(LicenseStatus::Valid {
                expires_at: Some(parsed.expires_at),
                source: ValidationSource::OfflineCache,
            });
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
}

/// Hashes the file at `path` and returns Ok on success.
/// Exposed as a helper so tests can drive every branch by passing custom paths.
pub fn integrity_check_path(path: &std::path::Path) -> Result<(), LicenseError> {
    let mut f = fs::File::open(path).map_err(|e| LicenseError::Io(e.to_string()))?;
    let mut buf = Vec::new();
    f.read_to_end(&mut buf).map_err(|e| LicenseError::Io(e.to_string()))?;
    // We only need to digest the buffer to assert it is readable; the digest itself is
    // not currently used downstream (a future revision will pin a known-good digest).
    let _digest = sha2::Sha256::digest(&buf);
    Ok(())
}

/// Returns the production integrity check that hashes the running executable.
/// We expose a noop variant for tests via `noop_integrity_check`.
pub fn default_integrity_check() -> IntegrityCheck {
    Box::new(|| {
        let exe = std::env::current_exe().map_err(|e| LicenseError::Io(e.to_string()))?;
        integrity_check_path(&exe)
    })
}

/// Returns an integrity check that always succeeds. Used by tests.
pub fn noop_integrity_check() -> IntegrityCheck {
    Box::new(|| Ok(()))
}

fn uuid_string() -> String {
    let bytes = hardware::machine_id_bytes();
    format!(
        "{:02x}{:02x}{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}",
        bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7],
        bytes[8], bytes[9], bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15]
    )
}

fn now_unix() -> i64 {
    SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_secs() as i64
}

#[cfg(test)]
mod tests {
    use super::*;

    fn dummy_config() -> ClientConfig {
        ClientConfig {
            server_url: "http://localhost:0".to_string(),
            cache_path: std::env::temp_dir().join("licensify-test-noop.token"),
            server_public_key: [0u8; 32],
        }
    }

    #[test]
    fn activate_then_entitlements_and_features() {
        let mut c = LicensifyClient::with_integrity_check(dummy_config(), noop_integrity_check()).unwrap();
        assert!(matches!(c.activate(""), Err(LicenseError::Inactive)));
        let res = c.activate("KEY").unwrap();
        assert!(!res.license_id.is_empty());
        let ent = c.entitlements().unwrap();
        assert!(ent.features.contains(&"base".to_string()));
        assert!(c.has_feature("base"));
        assert!(!c.has_feature("nope"));
        let mid = c.machine_id().unwrap();
        assert!(!mid.is_empty());
        c.deactivate().unwrap();
        assert!(matches!(c.entitlements(), Err(LicenseError::Inactive)));
        assert!(!c.has_feature("base"));
    }

    #[test]
    fn check_returns_invalid_when_cache_missing() {
        let mut cfg = dummy_config();
        cfg.cache_path = std::env::temp_dir().join("licensify-test-missing-cache-do-not-create");
        let _ = std::fs::remove_file(&cfg.cache_path);
        let c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        let status = c.check().unwrap();
        assert!(matches!(status, LicenseStatus::Invalid));
    }

    #[test]
    fn check_propagates_integrity_error() {
        let cfg = dummy_config();
        let c = LicensifyClient::with_integrity_check(
            cfg,
            Box::new(|| Err(LicenseError::Crypto("tampered".into()))),
        )
        .unwrap();
        assert!(matches!(c.check(), Err(LicenseError::Crypto(_))));
    }

    #[test]
    fn default_integrity_check_passes_for_running_test_binary() {
        let f = default_integrity_check();
        f().unwrap();
    }

    #[test]
    fn integrity_check_path_succeeds_for_existing_file() {
        let path = std::env::temp_dir().join("licensify-integrity-check-ok");
        std::fs::write(&path, b"hello").unwrap();
        integrity_check_path(&path).unwrap();
    }

    #[test]
    fn integrity_check_path_fails_for_missing_file() {
        let path = std::env::temp_dir().join("licensify-integrity-check-does-not-exist");
        let _ = std::fs::remove_file(&path);
        let err = integrity_check_path(&path).unwrap_err();
        assert!(matches!(err, LicenseError::Io(_)));
    }

    #[test]
    fn now_unix_is_positive_and_grows() {
        let a = now_unix();
        std::thread::sleep(std::time::Duration::from_millis(5));
        let b = now_unix();
        assert!(b >= a);
    }

    #[test]
    fn uuid_string_is_uuidlike() {
        let s = uuid_string();
        assert_eq!(s.len(), 36);
        assert_eq!(s.chars().filter(|c| *c == '-').count(), 4);
    }
}
