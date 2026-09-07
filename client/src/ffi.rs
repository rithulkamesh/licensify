// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/ffi.rs — C ABI (`licensify_*`) exports for embedding the Rust client.

use std::ffi::{c_char, CStr, CString};
use std::ptr;

use crate::{cert, ClientConfig, LicensifyClient};

/// Decodes a 64-char hex C string into 32 bytes. Returns `None` on any framing
/// error so callers can surface `false` rather than trusting a partial value.
fn hex32_from_c(ptr: *const c_char) -> Option<[u8; 32]> {
    if ptr.is_null() {
        return None;
    }
    let s = unsafe { CStr::from_ptr(ptr) }.to_string_lossy().into_owned();
    let bytes = hex::decode(s.trim()).ok()?;
    bytes.as_slice().try_into().ok()
}

/// Reads a 64-char hex value from an environment variable into 32 bytes.
fn hex32_from_env(key: &str) -> Option<[u8; 32]> {
    let s = std::env::var(key).ok()?;
    let bytes = hex::decode(s.trim()).ok()?;
    bytes.as_slice().try_into().ok()
}

#[repr(C)]
pub struct licensify_client_t {
    inner: LicensifyClient,
    last_error: Option<CString>,
}

#[repr(C)]
pub struct licensify_config_t {
    pub server_url: *const c_char,
    pub cache_path: *const c_char,
}

#[repr(C)]
pub struct licensify_result_t {
    pub ok: bool,
    pub message: *mut c_char,
}

#[repr(C)]
pub struct licensify_status_t {
    pub code: i32,
}

#[repr(C)]
#[derive(Copy, Clone)]
#[allow(non_camel_case_types)]
pub enum licensify_error_code_t {
    LICENSIFY_OK = 0,
    LICENSIFY_ERR_INVALID_ARGUMENT = 1,
    LICENSIFY_ERR_INITIALIZATION = 2,
    LICENSIFY_ERR_ACTIVATION = 3,
    LICENSIFY_ERR_CHECK = 4,
}

fn set_error(cli: &mut licensify_client_t, msg: impl Into<String>) {
    cli.last_error = Some(CString::new(msg.into()).unwrap_or_default());
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_new(config: *const licensify_config_t) -> *mut licensify_client_t {
    if config.is_null() {
        return ptr::null_mut();
    }
    let cfg = unsafe { &*config };
    let url = unsafe { CStr::from_ptr(cfg.server_url) }.to_string_lossy().into_owned();
    let cache = unsafe { CStr::from_ptr(cfg.cache_path) }.to_string_lossy().into_owned();
    // Zero-friction wiring for embedders that cannot call the setters below:
    // both the server token key and the pinned binary digest may be supplied
    // through the environment.
    let rcfg = ClientConfig {
        server_url: url,
        cache_path: cache.into(),
        server_public_key: hex32_from_env("LICENSIFY_SERVER_PUBLIC_KEY").unwrap_or([0_u8; 32]),
        expected_digest: hex32_from_env("LICENSIFY_EXPECTED_DIGEST"),
    };
    // `LicensifyClient::new` is currently infallible, so the Err arm is unreachable in
    // practice. Map a future failure to a NULL return for ABI stability.
    match LicensifyClient::new(rcfg) {
        Ok(inner) => Box::into_raw(Box::new(licensify_client_t { inner, last_error: None })),
        // grcov-excl-line: defensive path retained for ABI stability.
        Err(_) => ptr::null_mut(),
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_activate(client: *mut licensify_client_t, key: *const c_char) -> licensify_result_t {
    if client.is_null() || key.is_null() {
        return licensify_result_t { ok: false, message: CString::new("invalid arguments").unwrap_or_default().into_raw() };
    }
    let cli = unsafe { &mut *client };
    let key_s = unsafe { CStr::from_ptr(key) }.to_string_lossy().into_owned();
    match cli.inner.activate(&key_s) {
        Ok(_) => {
            cli.last_error = None;
            licensify_result_t { ok: true, message: CString::new("ok").unwrap_or_default().into_raw() }
        }
        Err(e) => {
            set_error(cli, e.to_string());
            licensify_result_t { ok: false, message: CString::new(e.to_string()).unwrap_or_default().into_raw() }
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_check(client: *mut licensify_client_t) -> licensify_status_t {
    if client.is_null() {
        return licensify_status_t { code: -1 };
    }
    let cli = unsafe { &mut *client };
    match cli.inner.check() {
        Ok(_) => {
            cli.last_error = None;
            licensify_status_t { code: 0 }
        }
        Err(e) => {
            set_error(cli, e.to_string());
            licensify_status_t { code: 1 }
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_has_feature(client: *mut licensify_client_t, feature: *const c_char) -> bool {
    if client.is_null() || feature.is_null() {
        return false;
    }
    let cli = unsafe { &mut *client };
    let feature_s = unsafe { CStr::from_ptr(feature) }.to_string_lossy().into_owned();
    cli.inner.has_feature(&feature_s)
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_activate_code(client: *mut licensify_client_t, key: *const c_char) -> licensify_error_code_t {
    if client.is_null() || key.is_null() {
        return licensify_error_code_t::LICENSIFY_ERR_INVALID_ARGUMENT;
    }
    let cli = unsafe { &mut *client };
    let key_s = unsafe { CStr::from_ptr(key) }.to_string_lossy().into_owned();
    match cli.inner.activate(&key_s) {
        Ok(_) => {
            cli.last_error = None;
            licensify_error_code_t::LICENSIFY_OK
        }
        Err(e) => {
            set_error(cli, e.to_string());
            licensify_error_code_t::LICENSIFY_ERR_ACTIVATION
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_check_code(client: *mut licensify_client_t, out_status_code: *mut i32) -> licensify_error_code_t {
    if client.is_null() || out_status_code.is_null() {
        return licensify_error_code_t::LICENSIFY_ERR_INVALID_ARGUMENT;
    }
    let cli = unsafe { &mut *client };
    match cli.inner.check() {
        Ok(status) => {
            cli.last_error = None;
            unsafe { *out_status_code = match status { crate::LicenseStatus::Valid { .. } => 0, _ => 1 } };
            licensify_error_code_t::LICENSIFY_OK
        }
        Err(e) => {
            set_error(cli, e.to_string());
            unsafe { *out_status_code = 1 };
            licensify_error_code_t::LICENSIFY_ERR_CHECK
        }
    }
}

/// Sets the server's Ed25519 token-signing public key (64 hex chars) used for
/// offline token verification. Returns `false` on a null client or malformed
/// hex. Optional: `activate` also learns this key from the server, and
/// `LICENSIFY_SERVER_PUBLIC_KEY` is honoured at construction time.
#[unsafe(no_mangle)]
pub extern "C" fn licensify_set_server_key(client: *mut licensify_client_t, hex_key: *const c_char) -> bool {
    if client.is_null() {
        return false;
    }
    let cli = unsafe { &mut *client };
    match hex32_from_c(hex_key) {
        Some(key) => {
            cli.inner.set_server_key(key);
            true
        }
        None => {
            set_error(cli, "server key must be 64 hex characters");
            false
        }
    }
}

/// Pins the expected SHA-256 (64 hex chars) of the host executable. Once set,
/// every `licensify_check` fails closed if the running binary's digest differs.
/// Passing a null/empty/invalid value clears the pin and returns `false`.
#[unsafe(no_mangle)]
pub extern "C" fn licensify_set_expected_digest(client: *mut licensify_client_t, hex_digest: *const c_char) -> bool {
    if client.is_null() {
        return false;
    }
    let cli = unsafe { &mut *client };
    match hex32_from_c(hex_digest) {
        Some(digest) => {
            cli.inner.set_expected_digest(Some(digest));
            true
        }
        None => {
            cli.inner.set_expected_digest(None);
            set_error(cli, "expected digest must be 64 hex characters");
            false
        }
    }
}

/// Verifies a `leaf → intermediate → root` Ed25519 certificate chain (raw DER).
/// Returns `0` on a valid chain, `1` on a validation failure, `-1` on bad
/// arguments. Standalone: does not require a client handle.
///
/// # Safety
/// Each pointer must be valid for reads of the paired length.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn licensify_verify_cert_chain(
    root: *const u8,
    root_len: usize,
    intermediate: *const u8,
    intermediate_len: usize,
    leaf: *const u8,
    leaf_len: usize,
) -> i32 {
    if root.is_null() || intermediate.is_null() || leaf.is_null() {
        return -1;
    }
    let root = std::slice::from_raw_parts(root, root_len);
    let intermediate = std::slice::from_raw_parts(intermediate, intermediate_len);
    let leaf = std::slice::from_raw_parts(leaf, leaf_len);
    match cert::verify_chain(root, intermediate, leaf) {
        Ok(()) => 0,
        Err(_) => 1,
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_last_error(client: *mut licensify_client_t) -> *const c_char {
    if client.is_null() {
        return ptr::null();
    }
    let cli = unsafe { &mut *client };
    match cli.last_error.as_ref() {
        Some(s) => s.as_ptr(),
        None => ptr::null(),
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_clear_error(client: *mut licensify_client_t) {
    if client.is_null() {
        return;
    }
    let cli = unsafe { &mut *client };
    cli.last_error = None;
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_string_free(s: *mut c_char) {
    if !s.is_null() {
        unsafe { drop(CString::from_raw(s)); }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn licensify_free(client: *mut licensify_client_t) {
    if !client.is_null() {
        unsafe { drop(Box::from_raw(client)); }
    }
}
