// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/cpp/tests/test_client.cpp — Catch2 tests covering every Licensify::Client method.

#include <catch2/catch_test_macros.hpp>

#include "licensify.hpp"

#include <string>
#include <utility>
#include <vector>

namespace {

class CapturingLogger : public Licensify::Logger {
 public:
  std::vector<std::string> debug_messages, info_messages, warn_messages, error_messages;
  void debug(const std::string& m) override { debug_messages.push_back(m); }
  void info(const std::string& m) override { info_messages.push_back(m); }
  void warn(const std::string& m) override { warn_messages.push_back(m); }
  void error(const std::string& m) override { error_messages.push_back(m); }
};

}  // namespace

TEST_CASE("create rejects empty configuration") {
  Licensify::Config cfg;
  cfg.server_url = "";
  cfg.cache_path = "/tmp/x";
  REQUIRE_THROWS_AS(Licensify::Client::create(cfg), Licensify::InitializationError);
  cfg.server_url = "x";
  cfg.cache_path = "";
  REQUIRE_THROWS_AS(Licensify::Client::create(cfg), Licensify::InitializationError);
}

TEST_CASE("activate rejects empty key without invoking native") {
  CapturingLogger log;
  Licensify::Config cfg;
  cfg.server_url = "http://localhost:0";
  cfg.cache_path = "/tmp/licensify-cpp.token";
  cfg.logger = &log;
  auto c = Licensify::Client::create(cfg);
  // Empty-key validation runs before any native or logging hooks, so the
  // logger should remain untouched until a real native error fires.
  REQUIRE_THROWS_AS(c.activate(""), Licensify::ActivationError);
  CHECK(log.error_messages.empty());
}

TEST_CASE("happy path activate / check / hasFeature") {
  CapturingLogger log;
  Licensify::Config cfg;
  cfg.server_url = "http://localhost:0";
  cfg.cache_path = "/tmp/licensify-cpp-happy.token";
  std::remove(cfg.cache_path.c_str());
  cfg.logger = &log;
  auto c = Licensify::Client::create(cfg);
  c.activate("LICENSE-KEY-DEV");
  int s = c.check();
  CHECK((s == 0 || s == 1));
  CHECK(c.hasFeature("base"));
  CHECK_FALSE(c.hasFeature(""));
  CHECK_FALSE(c.hasFeature("nope"));
  CHECK_FALSE(log.info_messages.empty());
  CHECK_FALSE(log.debug_messages.empty());
}

TEST_CASE("move construction and assignment transfer ownership") {
  Licensify::Config cfg;
  cfg.server_url = "http://localhost:0";
  cfg.cache_path = "/tmp/licensify-cpp-move.token";
  auto a = Licensify::Client::create(cfg);
  auto b = std::move(a);
  CHECK_NOTHROW(b.activate("LICENSE-KEY-DEV"));
  Licensify::Client c2;
  c2 = std::move(b);
  CHECK_NOTHROW(c2.check());
}

TEST_CASE("close is idempotent and after-close throws") {
  Licensify::Config cfg;
  cfg.server_url = "http://localhost:0";
  cfg.cache_path = "/tmp/licensify-cpp-close.token";
  auto c = Licensify::Client::create(cfg);
  c.close();
  c.close();
  REQUIRE_THROWS_AS(c.activate("KEY"), Licensify::InitializationError);
  REQUIRE_THROWS_AS(c.check(), Licensify::InitializationError);
  REQUIRE_THROWS_AS(c.hasFeature("base"), Licensify::InitializationError);
}

TEST_CASE("error code accessors expose native code") {
  Licensify::Config cfg;
  cfg.server_url = "http://localhost:0";
  cfg.cache_path = "/tmp/licensify-cpp-err.token";
  auto c = Licensify::Client::create(cfg);
  bool thrown = false;
  try {
    c.activate("");
  } catch (const Licensify::ActivationError& e) {
    thrown = true;
    CHECK(e.code() == LICENSIFY_ERR_INVALID_ARGUMENT);
  }
  CHECK(thrown);

  // CheckError code path is harder to reach without a malformed cache; we
  // construct an instance directly to exercise the accessor.
  Licensify::CheckError ce(LICENSIFY_ERR_CHECK, "boom");
  CHECK(ce.code() == LICENSIFY_ERR_CHECK);
}
