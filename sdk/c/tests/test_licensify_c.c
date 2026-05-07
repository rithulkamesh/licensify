/* SPDX-License-Identifier: MIT
 * Copyright (c) 2026 Rithul Kamesh
 * Author: Rithul Kamesh <hi@rithul.dev>
 * Description: sdk/c/tests/test_licensify_c.c — Test runner exercising every C ABI export.
 */

#include "licensify_c.h"
#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int g_fails = 0;

#define EXPECT(cond, msg)                                       \
  do {                                                          \
    if (!(cond)) {                                              \
      fprintf(stderr, "FAIL: %s (%s:%d)\n", msg, __FILE__, __LINE__); \
      g_fails++;                                                \
    }                                                           \
  } while (0)

static void test_null_safety(void) {
  EXPECT(licensify_new(NULL) == NULL, "new(NULL) returns NULL");
  licensify_free(NULL);
  licensify_clear_error(NULL);
  licensify_string_free(NULL);
  EXPECT(licensify_last_error(NULL) == NULL, "last_error(NULL) returns NULL");
  EXPECT(!licensify_has_feature(NULL, NULL), "has_feature(NULL,NULL) false");
  licensify_status_t st = licensify_check(NULL);
  EXPECT(st.code == -1, "check(NULL) returns -1");
  licensify_result_t res = licensify_activate(NULL, NULL);
  EXPECT(!res.ok, "activate(NULL,NULL) ok=false");
  licensify_string_free(res.message);
  int outc = 0;
  licensify_error_code_t code = licensify_check_code(NULL, &outc);
  EXPECT(code == LICENSIFY_ERR_INVALID_ARGUMENT, "check_code(NULL,&out) invalid arg");
  code = licensify_activate_code(NULL, "k");
  EXPECT(code == LICENSIFY_ERR_INVALID_ARGUMENT, "activate_code(NULL,k) invalid arg");
}

static void test_lifecycle(void) {
  licensify_config_t cfg = {"http://localhost:0", "/tmp/licensify-c-tests.token"};
  remove("/tmp/licensify-c-tests.token");
  licensify_client_t* c = licensify_new(&cfg);
  EXPECT(c != NULL, "new returns client");
  if (!c) return;

  // Empty key activate via struct API.
  licensify_result_t res = licensify_activate(c, "");
  EXPECT(!res.ok, "activate(\"\") fails");
  licensify_string_free(res.message);

  const char* msg = licensify_last_error(c);
  EXPECT(msg != NULL && strlen(msg) > 0, "last_error populated after failure");
  licensify_clear_error(c);
  EXPECT(licensify_last_error(c) == NULL, "clear_error wipes message");

  // Activate happy path.
  res = licensify_activate(c, "LICENSE-KEY-DEV");
  EXPECT(res.ok, "activate(KEY) ok");
  licensify_string_free(res.message);

  // Code-style activate.
  licensify_error_code_t code = licensify_activate_code(c, "LICENSE-KEY-DEV");
  EXPECT(code == LICENSIFY_OK, "activate_code(KEY) ok");
  code = licensify_activate_code(c, "");
  EXPECT(code == LICENSIFY_ERR_ACTIVATION, "activate_code(\"\") activation err");

  int status = -42;
  code = licensify_check_code(c, &status);
  EXPECT(code == LICENSIFY_OK, "check_code returns OK");
  EXPECT(status == 0 || status == 1, "check_code populates status");

  EXPECT(licensify_check_code(c, NULL) == LICENSIFY_ERR_INVALID_ARGUMENT,
         "check_code(c,NULL) invalid arg");

  licensify_status_t st = licensify_check(c);
  EXPECT(st.code == 0 || st.code == 1, "check struct populates code");

  EXPECT(licensify_has_feature(c, "base"), "has_feature base true");
  EXPECT(!licensify_has_feature(c, "missing"), "has_feature missing false");
  EXPECT(!licensify_has_feature(c, NULL), "has_feature(NULL) false");

  licensify_free(c);
}

int main(void) {
  test_null_safety();
  test_lifecycle();
  if (g_fails == 0) {
    printf("ok %d\n", 0);
    return 0;
  }
  fprintf(stderr, "%d test(s) failed\n", g_fails);
  return 1;
}
