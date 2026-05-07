// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/tests/ffi_tests.rs — Integration tests covering the C ABI exports for null-pointer guards and happy paths.

use std::ffi::{CStr, CString};
use std::ptr;

use licensify::ffi::*;

fn cstr(s: &str) -> CString {
    CString::new(s).unwrap()
}

fn make_config(url: &CString, cache: &CString) -> licensify_config_t {
    licensify_config_t {
        server_url: url.as_ptr(),
        cache_path: cache.as_ptr(),
    }
}

#[test]
fn licensify_new_returns_null_for_null_config() {
    let p = licensify_new(ptr::null());
    assert!(p.is_null());
}

#[test]
fn licensify_new_returns_client_for_valid_config() {
    let url = cstr("http://localhost:0");
    let cache = cstr("/tmp/licensify-ffi.token");
    let cfg = make_config(&url, &cache);
    let p = licensify_new(&cfg);
    assert!(!p.is_null());
    licensify_free(p);
}

#[test]
fn licensify_free_handles_null() {
    licensify_free(ptr::null_mut());
}

#[test]
fn licensify_activate_null_args_returns_invalid_argument_code() {
    let key = cstr("KEY");
    let code = licensify_activate_code(ptr::null_mut(), key.as_ptr());
    assert!(matches!(code, licensify_error_code_t::LICENSIFY_ERR_INVALID_ARGUMENT));
}

#[test]
fn licensify_activate_struct_null_returns_error_struct() {
    let res = licensify_activate(ptr::null_mut(), ptr::null());
    assert!(!res.ok);
    licensify_string_free(res.message);
}

#[test]
fn licensify_activate_happy_then_check() {
    let url = cstr("http://localhost:0");
    let cache = cstr("/tmp/licensify-ffi-happy.token");
    let _ = std::fs::remove_file("/tmp/licensify-ffi-happy.token");
    let cfg = make_config(&url, &cache);
    let p = licensify_new(&cfg);
    assert!(!p.is_null());

    let key = cstr("LICENSE-KEY-DEV");
    let res = licensify_activate(p, key.as_ptr());
    assert!(res.ok);
    licensify_string_free(res.message);

    let code = licensify_activate_code(p, key.as_ptr());
    assert!(matches!(code, licensify_error_code_t::LICENSIFY_OK));

    let mut status: i32 = 0;
    let code = licensify_check_code(p, &mut status);
    assert!(matches!(code, licensify_error_code_t::LICENSIFY_OK));
    // No cache file means status code should be 1 (Invalid).
    assert_eq!(status, 1);

    let st = licensify_check(p);
    assert!(st.code == 0 || st.code == 1);

    let feature = cstr("base");
    assert!(licensify_has_feature(p, feature.as_ptr()));
    let nope = cstr("nope");
    assert!(!licensify_has_feature(p, nope.as_ptr()));

    licensify_free(p);
}

#[test]
fn licensify_activate_empty_key_sets_error_then_clear() {
    let url = cstr("http://localhost:0");
    let cache = cstr("/tmp/licensify-ffi-emptykey.token");
    let cfg = make_config(&url, &cache);
    let p = licensify_new(&cfg);
    assert!(!p.is_null());

    let empty = cstr("");
    let res = licensify_activate(p, empty.as_ptr());
    assert!(!res.ok);
    licensify_string_free(res.message);

    let last = licensify_last_error(p);
    assert!(!last.is_null());
    let s = unsafe { CStr::from_ptr(last) }.to_string_lossy().into_owned();
    assert!(!s.is_empty());

    licensify_clear_error(p);
    assert!(licensify_last_error(p).is_null());

    licensify_free(p);
}

#[test]
fn licensify_activate_code_empty_key_returns_activation_error() {
    let url = cstr("http://localhost:0");
    let cache = cstr("/tmp/licensify-ffi-actcode.token");
    let cfg = make_config(&url, &cache);
    let p = licensify_new(&cfg);
    let empty = cstr("");
    let code = licensify_activate_code(p, empty.as_ptr());
    assert!(matches!(code, licensify_error_code_t::LICENSIFY_ERR_ACTIVATION));
    licensify_free(p);
}

#[test]
fn licensify_check_struct_null_returns_negative() {
    let st = licensify_check(ptr::null_mut());
    assert_eq!(st.code, -1);
}

#[test]
fn licensify_check_code_null_args_returns_invalid_argument() {
    let url = cstr("http://localhost:0");
    let cache = cstr("/tmp/licensify-ffi-checknull.token");
    let cfg = make_config(&url, &cache);
    let p = licensify_new(&cfg);
    let code = licensify_check_code(p, ptr::null_mut());
    assert!(matches!(code, licensify_error_code_t::LICENSIFY_ERR_INVALID_ARGUMENT));
    let code2 = licensify_check_code(ptr::null_mut(), ptr::null_mut());
    assert!(matches!(code2, licensify_error_code_t::LICENSIFY_ERR_INVALID_ARGUMENT));
    licensify_free(p);
}

#[test]
fn licensify_has_feature_null_args_returns_false() {
    assert!(!licensify_has_feature(ptr::null_mut(), ptr::null()));
}

#[test]
fn licensify_last_error_for_null_client_returns_null() {
    assert!(licensify_last_error(ptr::null_mut()).is_null());
}

#[test]
fn licensify_clear_error_handles_null_client() {
    licensify_clear_error(ptr::null_mut());
}

#[test]
fn licensify_string_free_handles_null() {
    licensify_string_free(ptr::null_mut());
}

#[test]
fn token_cache_round_trip() {
    use ed25519_dalek::{Signer, SigningKey};
    use sha2::Digest;

    let signing_key = SigningKey::from_bytes(&[7u8; 32]);
    let verifying_key = signing_key.verifying_key();
    let mut machine_id = [0u8; 32];
    machine_id.copy_from_slice(&sha2::Sha256::digest(b"mid")[..]);
    let now = 1_700_000_000i64;
    let mut tok = Vec::new();
    tok.push(1u8);
    tok.extend_from_slice(&[0x11u8; 16]);
    tok.extend_from_slice(&machine_id);
    tok.extend_from_slice(&now.to_be_bytes());
    tok.extend_from_slice(&(now + 60).to_be_bytes());
    tok.extend_from_slice(&(20u32).to_be_bytes());
    tok.extend_from_slice(b"{\"features\":[\"pro\"]}");
    tok.extend_from_slice(&[0x22u8; 32]);
    let sig = signing_key.sign(&tok);
    tok.extend_from_slice(&sig.to_bytes());
    let parsed =
        licensify::token::verify(&tok, &verifying_key.to_bytes(), now).expect("verify ok");
    assert_eq!(parsed.version, 1);
    assert_eq!(parsed.machine_id, machine_id);
}

#[test]
fn cache_encrypt_decrypt_machine_bound() {
    let prev = std::env::var("LICENSIFY_MACHINE_ID").ok();
    std::env::set_var("LICENSIFY_MACHINE_ID", "test-machine");
    let token = b"hello-token-bytes";
    let enc = licensify::cache::encrypt_token(token).unwrap();
    let dec = licensify::cache::decrypt_token(&enc).unwrap();
    assert_eq!(dec, token);
    std::env::set_var("LICENSIFY_MACHINE_ID", "different-machine");
    assert!(licensify::cache::decrypt_token(&enc).is_err());
    match prev {
        Some(v) => std::env::set_var("LICENSIFY_MACHINE_ID", v),
        None => std::env::remove_var("LICENSIFY_MACHINE_ID"),
    }
}
