/* SPDX-License-Identifier: MIT
 * Copyright (c) 2026 Rithul Kamesh
 * Author: Rithul Kamesh <hi@rithul.dev>
 * Description: client/include/licensify.h — Stable C ABI for the Licensify native client.
 */

#ifndef LICENSIFY_H
#define LICENSIFY_H

#include <stdbool.h>
#include <stddef.h>

typedef struct licensify_client_t licensify_client_t;

typedef struct licensify_config_t {
  const char* server_url;
  const char* cache_path;
} licensify_config_t;

typedef struct licensify_result_t {
  bool ok;
  char* message;
} licensify_result_t;

typedef struct licensify_status_t {
  int code;
} licensify_status_t;

typedef enum licensify_error_code_t {
  LICENSIFY_OK = 0,
  LICENSIFY_ERR_INVALID_ARGUMENT = 1,
  LICENSIFY_ERR_INITIALIZATION = 2,
  LICENSIFY_ERR_ACTIVATION = 3,
  LICENSIFY_ERR_CHECK = 4
} licensify_error_code_t;

licensify_client_t* licensify_new(const licensify_config_t* config);
licensify_result_t licensify_activate(licensify_client_t* client, const char* key);
licensify_status_t licensify_check(licensify_client_t* client);
bool licensify_has_feature(licensify_client_t* client, const char* feature);
void licensify_free(licensify_client_t* client);

// New, SDK-friendly APIs:
// - return explicit error codes (no struct return marshalling required)
// - expose last error message owned by the client (valid until next call on same client)
licensify_error_code_t licensify_activate_code(licensify_client_t* client, const char* key);
licensify_error_code_t licensify_check_code(licensify_client_t* client, int* out_status_code);
const char* licensify_last_error(licensify_client_t* client);
void licensify_clear_error(licensify_client_t* client);

// Free a string allocated by Licensify (e.g., licensify_result_t.message).
void licensify_string_free(char* s);

// Optional hardening APIs. Existing callers need not use them; the ABI above is
// unchanged.
//
// licensify_set_server_key: 64 hex chars of the server's Ed25519 token-signing
//   public key, used for offline token verification. Also learned from the
//   server during licensify_activate, or from LICENSIFY_SERVER_PUBLIC_KEY.
// licensify_set_expected_digest: 64 hex chars of the expected SHA-256 of the
//   host executable. Once set, licensify_check fails closed on mismatch. Also
//   settable via LICENSIFY_EXPECTED_DIGEST.
// Both return false on a null client or malformed hex.
bool licensify_set_server_key(licensify_client_t* client, const char* hex_key);
bool licensify_set_expected_digest(licensify_client_t* client, const char* hex_digest);

// Verify a leaf -> intermediate -> root Ed25519 certificate chain (raw DER).
// Returns 0 (valid), 1 (validation failed), or -1 (bad arguments).
int licensify_verify_cert_chain(const unsigned char* root, size_t root_len,
                                const unsigned char* intermediate, size_t intermediate_len,
                                const unsigned char* leaf, size_t leaf_len);

#endif
