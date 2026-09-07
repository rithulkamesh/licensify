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
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use base64::prelude::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

pub use crate::error::LicenseError;

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClientConfig {
    pub server_url: String,
    pub cache_path: PathBuf,
    /// Ed25519 public key the server signs license tokens with. Offline
    /// validation verifies cached tokens against this key. May be left zeroed and
    /// populated later via [`LicensifyClient::set_server_key`] or an online
    /// [`LicensifyClient::activate`] (which fetches the server's published key).
    pub server_public_key: [u8; 32],
    /// Optional SHA-256 of the host executable. When set, every
    /// [`LicensifyClient::check`] recomputes the running binary's digest and
    /// fails closed on mismatch — a real anti-tamper gate rather than the old
    /// no-op that hashed the file and threw the result away.
    #[serde(default)]
    pub expected_digest: Option<[u8; 32]>,
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
/// guard against tamper. The production implementation reads the running
/// executable, digests it, and — when a digest is pinned — fails closed on
/// mismatch. Tests inject a no-op variant.
pub type IntegrityCheck = Box<dyn Fn() -> Result<(), LicenseError> + Send + Sync>;

pub struct LicensifyClient {
    config: ClientConfig,
    license_key: Option<String>,
    entitlements_cache: Option<Entitlements>,
    integrity_check: IntegrityCheck,
    last_online_error: Option<String>,
}

impl LicensifyClient {
    pub fn new(config: ClientConfig) -> Result<Self, LicenseError> {
        let expected = config.expected_digest;
        Self::with_integrity_check(config, default_integrity_check(expected))
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
            last_online_error: None,
        })
    }

    /// Set/replace the server's token-signing public key at runtime.
    pub fn set_server_key(&mut self, key: [u8; 32]) {
        self.config.server_public_key = key;
    }

    /// Pin (or clear) the expected host-binary SHA-256. Rebuilds the integrity
    /// check so the new digest takes effect on the next `check`.
    pub fn set_expected_digest(&mut self, digest: Option<[u8; 32]>) {
        self.config.expected_digest = digest;
        self.integrity_check = default_integrity_check(digest);
    }

    /// The most recent error from an online activation attempt, if any. Online
    /// failures are non-fatal (activation still succeeds offline), so this is the
    /// only way to observe them.
    pub fn last_online_error(&self) -> Option<&str> {
        self.last_online_error.as_deref()
    }

    pub fn activate(&mut self, key: &str) -> Result<ActivationResult, LicenseError> {
        if key.is_empty() {
            return Err(LicenseError::Inactive);
        }
        self.license_key = Some(key.to_string());
        self.last_online_error = None;

        // Best-effort online activation: register with the server, verify the
        // issued certificate chain, pull a signed token, and persist it to the
        // machine-bound cache. Any failure here is non-fatal — the client still
        // activates with conservative local entitlements and can be re-checked
        // later once connectivity returns.
        let entitlements = match self.try_online_activation(key) {
            Ok(Some(ent)) => ent,
            Ok(None) => default_entitlements(),
            Err(e) => {
                self.last_online_error = Some(e.to_string());
                default_entitlements()
            }
        };

        self.entitlements_cache = Some(entitlements.clone());
        Ok(ActivationResult { license_id: uuid_string(), entitlements })
    }

    fn try_online_activation(&mut self, key: &str) -> Result<Option<Entitlements>, LicenseError> {
        let base = self.config.server_url.trim_end_matches('/').to_string();
        if base.is_empty() {
            return Ok(None);
        }
        let http = reqwest::blocking::Client::builder()
            .timeout(Duration::from_secs(4))
            .build()
            .map_err(|e| LicenseError::Network(e.to_string()))?;

        // 1. Learn the server's token-signing key.
        let key_hex = http
            .get(format!("{base}/v1/.well-known/token-key"))
            .send()
            .and_then(|r| r.error_for_status())
            .and_then(|r| r.text())
            .map_err(|e| LicenseError::Network(e.to_string()))?;
        let key_bytes = hex::decode(key_hex.trim()).map_err(|e| LicenseError::Crypto(e.to_string()))?;
        let server_pk: [u8; 32] = key_bytes
            .as_slice()
            .try_into()
            .map_err(|_| LicenseError::Crypto("server token key is not 32 bytes".into()))?;
        self.config.server_public_key = server_pk;

        let machine_id = hardware::machine_id_bytes();
        let machine_b64 = BASE64_STANDARD.encode(machine_id);

        // 2. Activate: hand the server an opaque registration upload; receive a
        //    leaf/intermediate certificate pair.
        let reg_upload = opaque::registration_request(key, &machine_id);
        let act: ActivateResponse = http
            .post(format!("{base}/v1/activate"))
            .json(&serde_json::json!({
                "license_key": key,
                "machine_id": machine_b64,
                "opaque_registration_upload": BASE64_STANDARD.encode(reg_upload),
                "hardware_components": {},
            }))
            .send()
            .and_then(|r| r.error_for_status())
            .map_err(|e| LicenseError::Network(e.to_string()))?
            .json()
            .map_err(|e| LicenseError::Network(e.to_string()))?;

        // 3. Verify the issued chain against the server's published root.
        if let (Some(leaf_b64), Some(inter_b64)) = (&act.leaf_certificate, &act.intermediate_certificate) {
            let leaf = BASE64_STANDARD
                .decode(leaf_b64)
                .map_err(|e| LicenseError::Crypto(e.to_string()))?;
            let inter = BASE64_STANDARD
                .decode(inter_b64)
                .map_err(|e| LicenseError::Crypto(e.to_string()))?;
            let root = http
                .get(format!("{base}/v1/.well-known/ca"))
                .send()
                .and_then(|r| r.error_for_status())
                .and_then(|r| r.bytes())
                .map_err(|e| LicenseError::Network(e.to_string()))?;
            cert::verify_chain(&root, &inter, &leaf)?;
        }

        // 4. Validate: exchange an opaque login request for a signed token.
        let login_req = opaque::login_request(key, &machine_id);
        let val: ValidateResponse = http
            .post(format!("{base}/v1/validate"))
            .json(&serde_json::json!({
                "license_key": key,
                "machine_id": machine_b64,
                "opaque_login_request": BASE64_STANDARD.encode(login_req),
                "client_nonce": BASE64_STANDARD.encode(b"licensify-client"),
            }))
            .send()
            .and_then(|r| r.error_for_status())
            .map_err(|e| LicenseError::Network(e.to_string()))?
            .json()
            .map_err(|e| LicenseError::Network(e.to_string()))?;

        let token_bytes = BASE64_STANDARD
            .decode(val.license_token.trim())
            .map_err(|e| LicenseError::Crypto(e.to_string()))?;

        // 5. Verify signature + expiry + machine binding, then persist.
        let now = now_unix()?;
        let parsed = token::verify(&token_bytes, &self.config.server_public_key, now)?;
        if !ct_eq(&parsed.machine_id, &machine_binding_hash()) {
            return Err(LicenseError::MachineMismatch);
        }
        let encrypted = cache::encrypt_token(&token_bytes)?;
        fs::write(&self.config.cache_path, &encrypted).map_err(|e| LicenseError::Io(e.to_string()))?;
        write_rollback_marker(&self.config.cache_path, parsed.issued_at)?;

        Ok(parse_entitlements(&parsed.entitlements).or_else(|| Some(default_entitlements())))
    }

    pub fn check(&mut self) -> Result<LicenseStatus, LicenseError> {
        // Anti-tamper gate first: a pinned-digest mismatch fails the whole check.
        (self.integrity_check)()?;

        let now = now_unix()?;

        let encrypted = match fs::read(&self.config.cache_path) {
            Ok(bytes) => bytes,
            Err(_) => return Ok(LicenseStatus::Invalid),
        };

        let token_bytes = match cache::decrypt_token(&encrypted) {
            Ok(bytes) => bytes,
            Err(_) => return Ok(LicenseStatus::Invalid),
        };

        let parsed = match token::verify(&token_bytes, &self.config.server_public_key, now) {
            Ok(parsed) => parsed,
            Err(LicenseError::Expired) => return Ok(LicenseStatus::Expired),
            Err(_) => return Ok(LicenseStatus::Invalid),
        };

        // Machine binding: the token must be bound to *this* machine's
        // fingerprint. Previously never checked — a token lifted from another
        // host would validate.
        if !ct_eq(&parsed.machine_id, &machine_binding_hash()) {
            return Ok(LicenseStatus::Invalid);
        }

        // Rollback protection: reject a token older than the newest one we have
        // already honoured on this machine (clock rollback / stale re-seed).
        let marker_path = rollback_marker_path(&self.config.cache_path);
        if let Some(seen) = read_rollback_marker(&marker_path) {
            if parsed.issued_at < seen - token::CLOCK_SKEW_SECS {
                return Ok(LicenseStatus::Invalid);
            }
        }
        write_rollback_marker(&self.config.cache_path, parsed.issued_at)?;

        // Surface the *signed* entitlements so `has_feature`/`entitlements`
        // reflect what the server actually granted, not the activation-time
        // placeholder.
        if let Some(ent) = parse_entitlements(&parsed.entitlements) {
            self.entitlements_cache = Some(ent);
        }

        Ok(LicenseStatus::Valid {
            expires_at: Some(parsed.expires_at),
            source: ValidationSource::OfflineCache,
        })
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
        let _ = fs::remove_file(&self.config.cache_path);
        let _ = fs::remove_file(rollback_marker_path(&self.config.cache_path));
        Ok(())
    }

    pub fn machine_id(&self) -> Result<String, LicenseError> {
        Ok(hardware::machine_id_hex())
    }
}

#[derive(Deserialize)]
struct ActivateResponse {
    #[serde(default)]
    leaf_certificate: Option<String>,
    #[serde(default)]
    intermediate_certificate: Option<String>,
}

#[derive(Deserialize)]
struct ValidateResponse {
    license_token: String,
}

fn default_entitlements() -> Entitlements {
    Entitlements {
        license_type: LicenseType::Perpetual,
        features: vec!["base".to_string()],
        seat_count: None,
        max_activations: None,
        trial_ends_at: None,
        subscription_expires_at: None,
        offline_grace_days: 7,
        custom_metadata: HashMap::new(),
    }
}

/// Lenient parse of the server's entitlements JSON (snake_case, nullable fields).
/// Kept deliberately tolerant so minor client/server drift degrades gracefully
/// instead of dropping a validated token on the floor.
fn parse_entitlements(bytes: &[u8]) -> Option<Entitlements> {
    let v: serde_json::Value = serde_json::from_slice(bytes).ok()?;
    let obj = v.as_object()?;

    let features = obj
        .get("features")
        .and_then(|f| f.as_array())
        .map(|arr| arr.iter().filter_map(|x| x.as_str().map(String::from)).collect())
        .unwrap_or_default();

    let license_type = match obj.get("license_type").and_then(|x| x.as_str()) {
        Some("perpetual") => LicenseType::Perpetual,
        Some("subscription") => LicenseType::Subscription,
        Some("floating_seat") => LicenseType::FloatingSeat,
        Some("trial") => LicenseType::Trial,
        _ => LicenseType::FeatureFlagged,
    };

    let custom_metadata = obj
        .get("custom_metadata")
        .and_then(|x| x.as_object())
        .map(|m| m.iter().map(|(k, val)| (k.clone(), val.clone())).collect())
        .unwrap_or_default();

    Some(Entitlements {
        license_type,
        features,
        seat_count: obj.get("seat_count").and_then(|x| x.as_u64()).map(|n| n as u32),
        max_activations: obj.get("max_activations").and_then(|x| x.as_u64()).map(|n| n as u32),
        trial_ends_at: obj.get("trial_ends_at").and_then(|x| x.as_i64()),
        subscription_expires_at: obj.get("subscription_expires_at").and_then(|x| x.as_i64()),
        offline_grace_days: obj.get("offline_grace_days").and_then(|x| x.as_u64()).unwrap_or(7) as u32,
        custom_metadata,
    })
}

/// The machine-binding value embedded in server-issued tokens: `SHA-256` of this
/// host's hardware fingerprint. Mirrors the server's `sha256(machine_id)`.
fn machine_binding_hash() -> [u8; 32] {
    Sha256::digest(hardware::machine_id_bytes()).into()
}

/// Length-checked, branch-free byte comparison.
fn ct_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for (x, y) in a.iter().zip(b.iter()) {
        diff |= x ^ y;
    }
    diff == 0
}

fn rollback_marker_path(cache: &Path) -> PathBuf {
    let mut raw = cache.as_os_str().to_os_string();
    raw.push(".seen");
    PathBuf::from(raw)
}

fn read_rollback_marker(path: &Path) -> Option<i64> {
    fs::read_to_string(path).ok()?.trim().parse().ok()
}

fn write_rollback_marker(cache: &Path, issued_at: i64) -> Result<(), LicenseError> {
    let path = rollback_marker_path(cache);
    let prev = read_rollback_marker(&path).unwrap_or(i64::MIN);
    let newest = issued_at.max(prev);
    fs::write(&path, newest.to_string()).map_err(|e| LicenseError::Io(e.to_string()))
}

/// Hashes the file at `path`; returns its SHA-256. Exposed so tests can drive
/// every branch with custom paths.
pub fn integrity_check_path(path: &std::path::Path) -> Result<(), LicenseError> {
    sha256_file(path).map(|_| ())
}

fn sha256_file(path: &std::path::Path) -> Result<[u8; 32], LicenseError> {
    let mut f = fs::File::open(path).map_err(|e| LicenseError::Io(e.to_string()))?;
    let mut buf = Vec::new();
    f.read_to_end(&mut buf).map_err(|e| LicenseError::Io(e.to_string()))?;
    Ok(Sha256::digest(&buf).into())
}

/// Production integrity check: digests the running executable and, when
/// `expected` is `Some`, fails closed on any mismatch. When `expected` is `None`
/// it only asserts the binary is readable (a pinned digest is opt-in because a
/// binary cannot contain its own hash without a post-build patch step).
pub fn default_integrity_check(expected: Option<[u8; 32]>) -> IntegrityCheck {
    Box::new(move || {
        let exe = std::env::current_exe().map_err(|e| LicenseError::Io(e.to_string()))?;
        let digest = sha256_file(&exe)?;
        match expected {
            Some(want) if !ct_eq(&digest, &want) => {
                Err(LicenseError::Crypto("host binary integrity check failed".into()))
            }
            _ => Ok(()),
        }
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

fn now_unix() -> Result<i64, LicenseError> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .map_err(|_| LicenseError::Crypto("system clock is before the Unix epoch".into()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};
    use serial_test::serial;

    /// Pin the hardware fingerprint so cache round-trips are deterministic and
    /// independent of whatever the other (env-mutating) hardware tests are doing.
    fn pin_machine_id() {
        for k in ["LICENSIFY_DISK_SERIAL", "LICENSIFY_BOARD_UUID", "LICENSIFY_MAC", "LICENSIFY_GPU_ID", "LICENSIFY_TPM_EK_SEED"] {
            std::env::remove_var(k);
        }
        std::env::set_var("LICENSIFY_MACHINE_ID", "licensify-lib-test-machine");
    }

    fn dummy_config() -> ClientConfig {
        ClientConfig {
            server_url: String::new(),
            cache_path: std::env::temp_dir().join("licensify-test-noop.token"),
            server_public_key: [0u8; 32],
            expected_digest: None,
        }
    }

    fn signed_token(signing: &SigningKey, machine_id: [u8; 32], ent: &[u8], issued_at: i64, ttl: i64) -> Vec<u8> {
        let mut out = Vec::new();
        out.push(1u8);
        out.extend_from_slice(&[0x11u8; 16]);
        out.extend_from_slice(&machine_id);
        out.extend_from_slice(&issued_at.to_be_bytes());
        out.extend_from_slice(&(issued_at + ttl).to_be_bytes());
        out.extend_from_slice(&(ent.len() as u32).to_be_bytes());
        out.extend_from_slice(ent);
        out.extend_from_slice(&[0x22u8; 32]);
        let sig = signing.sign(&out);
        out.extend_from_slice(&sig.to_bytes());
        out
    }

    /// Writes a valid, machine-bound, encrypted cache for the current machine id
    /// and returns (config, verifying_key_bytes).
    fn seed_valid_cache(dir: &Path, ent_json: &[u8]) -> (ClientConfig, [u8; 32]) {
        pin_machine_id();
        let signing = SigningKey::from_bytes(&[9u8; 32]);
        let vk = signing.verifying_key().to_bytes();
        let now = now_unix().unwrap();
        let tok = signed_token(&signing, machine_binding_hash(), ent_json, now - 10, 3600);
        let enc = cache::encrypt_token(&tok).unwrap();
        let cache_path = dir.join("valid.token");
        fs::write(&cache_path, enc).unwrap();
        let _ = fs::remove_file(rollback_marker_path(&cache_path));
        (
            ClientConfig {
                server_url: String::new(),
                cache_path,
                server_public_key: vk,
                expected_digest: None,
            },
            vk,
        )
    }

    #[test]
    #[serial]
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
    fn activate_records_online_error_for_unreachable_server() {
        let mut cfg = dummy_config();
        cfg.server_url = "http://127.0.0.1:0".to_string();
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        c.activate("KEY").unwrap();
        assert!(c.last_online_error().is_some());
    }

    /// Minimal one-shot HTTP/1.1 server for exercising the online activation
    /// path. Handles exactly `expected` requests (each on its own connection,
    /// `Connection: close`) then exits. `route` maps a request path to a
    /// `(status, content_type, body)` triple.
    fn spawn_mock_server<F>(expected: usize, route: F) -> String
    where
        F: Fn(&str) -> (u16, &'static str, Vec<u8>) + Send + 'static,
    {
        use std::io::{Read as _, Write as _};
        use std::net::TcpListener;

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        std::thread::spawn(move || {
            for _ in 0..expected {
                let (mut stream, _) = listener.accept().unwrap();
                let mut buf = Vec::new();
                let mut tmp = [0u8; 1024];
                // Read headers.
                loop {
                    let n = stream.read(&mut tmp).unwrap();
                    if n == 0 {
                        break;
                    }
                    buf.extend_from_slice(&tmp[..n]);
                    if let Some(pos) = buf.windows(4).position(|w| w == b"\r\n\r\n") {
                        let header = String::from_utf8_lossy(&buf[..pos]).to_string();
                        let content_len = header
                            .lines()
                            .find_map(|l| l.to_ascii_lowercase().strip_prefix("content-length:").map(|v| v.trim().parse::<usize>().unwrap_or(0)))
                            .unwrap_or(0);
                        let have_body = buf.len() - (pos + 4);
                        let mut remaining = content_len.saturating_sub(have_body);
                        while remaining > 0 {
                            let n = stream.read(&mut tmp).unwrap();
                            if n == 0 {
                                break;
                            }
                            remaining -= n.min(remaining);
                        }
                        let path = header.lines().next().unwrap().split_whitespace().nth(1).unwrap_or("/").to_string();
                        let (status, ctype, body) = route(&path);
                        let resp = format!(
                            "HTTP/1.1 {status} OK\r\nContent-Type: {ctype}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                            body.len()
                        );
                        stream.write_all(resp.as_bytes()).unwrap();
                        stream.write_all(&body).unwrap();
                        stream.flush().unwrap();
                        break;
                    }
                }
            }
        });
        format!("http://{addr}")
    }

    #[test]
    #[serial]
    fn activate_online_persists_a_machine_bound_cache() {
        pin_machine_id();
        let signing = SigningKey::from_bytes(&[3u8; 32]);
        let pk_hex = hex::encode(signing.verifying_key().to_bytes());
        let now = now_unix().unwrap();
        let token = signed_token(
            &signing,
            machine_binding_hash(),
            br#"{"license_type":"subscription","features":["base","pro"],"offline_grace_days":9}"#,
            now - 5,
            3600,
        );
        let token_b64 = BASE64_STANDARD.encode(&token);

        let url = spawn_mock_server(3, move |path| {
            if path.contains("token-key") {
                (200, "text/plain", pk_hex.clone().into_bytes())
            } else if path.contains("/v1/activate") {
                (200, "application/json", b"{}".to_vec())
            } else if path.contains("/v1/validate") {
                (200, "application/json", format!("{{\"license_token\":\"{token_b64}\"}}").into_bytes())
            } else {
                (404, "text/plain", b"no".to_vec())
            }
        });

        let dir = std::env::temp_dir().join("licensify-online-activate");
        let _ = fs::create_dir_all(&dir);
        let cache_path = dir.join("c.token");
        let _ = fs::remove_file(&cache_path);
        let _ = fs::remove_file(rollback_marker_path(&cache_path));

        let cfg = ClientConfig {
            server_url: url,
            cache_path: cache_path.clone(),
            server_public_key: [0u8; 32],
            expected_digest: None,
        };
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        let res = c.activate("LICENSE-KEY-DEV").unwrap();
        assert!(c.last_online_error().is_none(), "online error: {:?}", c.last_online_error());
        assert!(matches!(res.entitlements.license_type, LicenseType::Subscription));
        assert_eq!(res.entitlements.offline_grace_days, 9);
        assert!(cache_path.exists());

        // The persisted cache now validates fully offline.
        let st = c.check().unwrap();
        assert!(matches!(st, LicenseStatus::Valid { source: ValidationSource::OfflineCache, .. }));
        assert!(c.has_feature("pro"));
    }

    #[test]
    #[serial]
    fn activate_online_verifies_issued_cert_chain() {
        use rcgen::{BasicConstraints as RcBc, CertificateParams, IsCa, KeyPair, PKCS_ED25519};
        pin_machine_id();

        let root_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let mut rp = CertificateParams::new(Vec::<String>::new()).unwrap();
        rp.is_ca = IsCa::Ca(RcBc::Unconstrained);
        let root = rp.self_signed(&root_key).unwrap();
        let int_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let mut ip = CertificateParams::new(Vec::<String>::new()).unwrap();
        ip.is_ca = IsCa::Ca(RcBc::Constrained(0));
        let inter = ip.signed_by(&int_key, &root, &root_key).unwrap();
        let leaf_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let lp = CertificateParams::new(vec!["leaf".to_string()]).unwrap();
        let leaf = lp.signed_by(&leaf_key, &inter, &int_key).unwrap();

        let root_der = root.der().to_vec();
        let leaf_b64 = BASE64_STANDARD.encode(leaf.der());
        let inter_b64 = BASE64_STANDARD.encode(inter.der());

        let signing = SigningKey::from_bytes(&[4u8; 32]);
        let pk_hex = hex::encode(signing.verifying_key().to_bytes());
        let now = now_unix().unwrap();
        let token = signed_token(&signing, machine_binding_hash(), br#"{"features":["base"]}"#, now - 5, 3600);
        let token_b64 = BASE64_STANDARD.encode(&token);

        let url = spawn_mock_server(4, move |path| {
            if path.contains("token-key") {
                (200, "text/plain", pk_hex.clone().into_bytes())
            } else if path.contains("/v1/activate") {
                (
                    200,
                    "application/json",
                    format!("{{\"leaf_certificate\":\"{leaf_b64}\",\"intermediate_certificate\":\"{inter_b64}\"}}").into_bytes(),
                )
            } else if path.contains("well-known/ca") {
                (200, "application/octet-stream", root_der.clone())
            } else if path.contains("/v1/validate") {
                (200, "application/json", format!("{{\"license_token\":\"{token_b64}\"}}").into_bytes())
            } else {
                (404, "text/plain", b"no".to_vec())
            }
        });

        let dir = std::env::temp_dir().join("licensify-online-chain");
        let _ = fs::create_dir_all(&dir);
        let cache_path = dir.join("c.token");
        let _ = fs::remove_file(&cache_path);
        let _ = fs::remove_file(rollback_marker_path(&cache_path));

        let cfg = ClientConfig {
            server_url: url,
            cache_path,
            server_public_key: [0u8; 32],
            expected_digest: None,
        };
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        c.activate("LICENSE-KEY-DEV").unwrap();
        assert!(c.last_online_error().is_none(), "online error: {:?}", c.last_online_error());
        assert!(matches!(c.check().unwrap(), LicenseStatus::Valid { .. }));
    }

    #[test]
    #[serial]
    fn activate_online_reports_rejected_cert_chain() {
        pin_machine_id();
        // activate returns certs that are not DER -> verify_chain fails -> online
        // activation errors (non-fatal) and no cache is written.
        let signing = SigningKey::from_bytes(&[5u8; 32]);
        let pk_hex = hex::encode(signing.verifying_key().to_bytes());
        let url = spawn_mock_server(3, move |path| {
            if path.contains("token-key") {
                (200, "text/plain", pk_hex.clone().into_bytes())
            } else if path.contains("/v1/activate") {
                (
                    200,
                    "application/json",
                    br#"{"leaf_certificate":"bm90LWRlcg==","intermediate_certificate":"bm90LWRlcg=="}"#.to_vec(),
                )
            } else {
                (200, "application/octet-stream", b"not-der".to_vec())
            }
        });
        let dir = std::env::temp_dir().join("licensify-online-badchain");
        let _ = fs::create_dir_all(&dir);
        let cache_path = dir.join("c.token");
        let _ = fs::remove_file(&cache_path);
        let cfg = ClientConfig {
            server_url: url,
            cache_path: cache_path.clone(),
            server_public_key: [0u8; 32],
            expected_digest: None,
        };
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        c.activate("KEY").unwrap();
        assert!(c.last_online_error().is_some());
        assert!(!cache_path.exists());
    }

    #[test]
    fn check_returns_invalid_when_cache_missing() {
        let mut cfg = dummy_config();
        cfg.cache_path = std::env::temp_dir().join("licensify-test-missing-cache-do-not-create");
        let _ = std::fs::remove_file(&cfg.cache_path);
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        assert!(matches!(c.check().unwrap(), LicenseStatus::Invalid));
    }

    #[test]
    fn check_propagates_integrity_error() {
        let c = LicensifyClient::with_integrity_check(
            dummy_config(),
            Box::new(|| Err(LicenseError::Crypto("tampered".into()))),
        );
        assert!(matches!(c.unwrap().check(), Err(LicenseError::Crypto(_))));
    }

    #[test]
    #[serial]
    fn check_validates_a_seeded_machine_bound_cache() {
        let dir = std::env::temp_dir().join("licensify-check-valid");
        let _ = fs::create_dir_all(&dir);
        let (cfg, _vk) = seed_valid_cache(&dir, b"{\"license_type\":\"perpetual\",\"features\":[\"base\",\"pro\"]}");
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        let st = c.check().unwrap();
        assert!(matches!(st, LicenseStatus::Valid { source: ValidationSource::OfflineCache, .. }));
        // Signed entitlements are now surfaced.
        assert!(c.has_feature("pro"));
        assert!(matches!(c.entitlements().unwrap().license_type, LicenseType::Perpetual));
    }

    #[test]
    #[serial]
    fn check_rejects_cache_signed_by_a_foreign_key() {
        let dir = std::env::temp_dir().join("licensify-check-foreign");
        let _ = fs::create_dir_all(&dir);
        let (mut cfg, _vk) = seed_valid_cache(&dir, b"{\"features\":[]}");
        cfg.server_public_key = [0u8; 32]; // wrong key
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        assert!(matches!(c.check().unwrap(), LicenseStatus::Invalid));
    }

    #[test]
    #[serial]
    fn check_rejects_token_bound_to_another_machine() {
        pin_machine_id();
        let dir = std::env::temp_dir().join("licensify-check-othermachine");
        let _ = fs::create_dir_all(&dir);
        let signing = SigningKey::from_bytes(&[9u8; 32]);
        let vk = signing.verifying_key().to_bytes();
        let now = now_unix().unwrap();
        let tok = signed_token(&signing, [0x55u8; 32], b"{\"features\":[]}", now - 10, 3600);
        let enc = cache::encrypt_token(&tok).unwrap();
        let cache_path = dir.join("m.token");
        fs::write(&cache_path, enc).unwrap();
        let cfg = ClientConfig {
            server_url: String::new(),
            cache_path,
            server_public_key: vk,
            expected_digest: None,
        };
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        assert!(matches!(c.check().unwrap(), LicenseStatus::Invalid));
    }

    #[test]
    #[serial]
    fn check_reports_expired_token() {
        pin_machine_id();
        let dir = std::env::temp_dir().join("licensify-check-expired");
        let _ = fs::create_dir_all(&dir);
        let signing = SigningKey::from_bytes(&[9u8; 32]);
        let vk = signing.verifying_key().to_bytes();
        let now = now_unix().unwrap();
        let tok = signed_token(&signing, machine_binding_hash(), b"{\"features\":[]}", now - 10_000, 100);
        let enc = cache::encrypt_token(&tok).unwrap();
        let cache_path = dir.join("e.token");
        fs::write(&cache_path, enc).unwrap();
        let cfg = ClientConfig {
            server_url: String::new(),
            cache_path,
            server_public_key: vk,
            expected_digest: None,
        };
        let mut c = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        assert!(matches!(c.check().unwrap(), LicenseStatus::Expired));
    }

    #[test]
    #[serial]
    fn check_rejects_rolled_back_token() {
        pin_machine_id();
        let dir = std::env::temp_dir().join("licensify-check-rollback");
        let _ = fs::create_dir_all(&dir);
        let signing = SigningKey::from_bytes(&[9u8; 32]);
        let vk = signing.verifying_key().to_bytes();
        let now = now_unix().unwrap();
        let cache_path = dir.join("r.token");
        let cfg = ClientConfig {
            server_url: String::new(),
            cache_path: cache_path.clone(),
            server_public_key: vk,
            expected_digest: None,
        };

        // First: honour a recent token.
        let fresh = signed_token(&signing, machine_binding_hash(), b"{\"features\":[]}", now - 10, 3600);
        fs::write(&cache_path, cache::encrypt_token(&fresh).unwrap()).unwrap();
        let _ = fs::remove_file(rollback_marker_path(&cache_path));
        let mut c = LicensifyClient::with_integrity_check(cfg.clone(), noop_integrity_check()).unwrap();
        assert!(matches!(c.check().unwrap(), LicenseStatus::Valid { .. }));

        // Then: swap in a much older (but still unexpired) token — must be rejected.
        let stale = signed_token(&signing, machine_binding_hash(), b"{\"features\":[]}", now - 100_000, 1_000_000);
        fs::write(&cache_path, cache::encrypt_token(&stale).unwrap()).unwrap();
        let mut c2 = LicensifyClient::with_integrity_check(cfg, noop_integrity_check()).unwrap();
        assert!(matches!(c2.check().unwrap(), LicenseStatus::Invalid));
    }

    #[test]
    fn set_expected_digest_enforces_binary_pin() {
        let mut c = LicensifyClient::with_integrity_check(dummy_config(), noop_integrity_check()).unwrap();
        c.set_expected_digest(Some([0u8; 32])); // deliberately wrong
        assert!(matches!(c.check(), Err(LicenseError::Crypto(_))));
        c.set_expected_digest(None);
        // With the pin cleared the integrity gate passes and we fall through to
        // the (missing) cache path.
        assert!(matches!(c.check().unwrap(), LicenseStatus::Invalid));
    }

    #[test]
    fn set_server_key_updates_config() {
        let mut c = LicensifyClient::with_integrity_check(dummy_config(), noop_integrity_check()).unwrap();
        c.set_server_key([7u8; 32]);
        assert_eq!(c.config.server_public_key, [7u8; 32]);
    }

    #[test]
    fn default_integrity_check_passes_without_pin_and_fails_with_bad_pin() {
        default_integrity_check(None)().unwrap();
        assert!(matches!(
            default_integrity_check(Some([1u8; 32]))(),
            Err(LicenseError::Crypto(_))
        ));
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
        assert!(matches!(integrity_check_path(&path).unwrap_err(), LicenseError::Io(_)));
    }

    #[test]
    fn parse_entitlements_is_lenient() {
        let ent = parse_entitlements(
            br#"{"license_type":"subscription","features":["a","b"],"seat_count":5,"custom_metadata":null}"#,
        )
        .unwrap();
        assert!(matches!(ent.license_type, LicenseType::Subscription));
        assert_eq!(ent.features, vec!["a", "b"]);
        assert_eq!(ent.seat_count, Some(5));
        assert_eq!(ent.offline_grace_days, 7);
        assert!(parse_entitlements(b"not json").is_none());
        assert!(parse_entitlements(b"[]").is_none());
        // Unknown license type falls back rather than failing.
        assert!(matches!(
            parse_entitlements(br#"{"license_type":"???"}"#).unwrap().license_type,
            LicenseType::FeatureFlagged
        ));
    }

    #[test]
    fn now_unix_is_positive_and_grows() {
        let a = now_unix().unwrap();
        std::thread::sleep(std::time::Duration::from_millis(5));
        let b = now_unix().unwrap();
        assert!(b >= a);
    }

    #[test]
    fn uuid_string_is_uuidlike() {
        let s = uuid_string();
        assert_eq!(s.len(), 36);
        assert_eq!(s.chars().filter(|c| *c == '-').count(), 4);
    }

    #[test]
    fn ct_eq_matches_semantics() {
        assert!(ct_eq(b"abc", b"abc"));
        assert!(!ct_eq(b"abc", b"abd"));
        assert!(!ct_eq(b"abc", b"ab"));
    }
}
