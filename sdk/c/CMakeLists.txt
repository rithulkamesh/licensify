# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Rithul Kamesh
# Author: Rithul Kamesh <hi@rithul.dev>
# Description: sdk/c/CMakeLists.txt — CMake build for the Licensify C SDK and its coverage-instrumented test runner.

cmake_minimum_required(VERSION 3.20)
project(licensify_c LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

# The header-only "C SDK" lives in `include/`; tests link directly against the
# Rust-built `liblicensify` (`.so`/`.dylib`/`.dll`) at `target/{debug,release}`.
set(CLIENT_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../client/include")
set(LIBLICENSIFY_DEBUG_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../target/debug")
set(LIBLICENSIFY_RELEASE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../target/release")

option(LICENSIFY_C_COVERAGE "Build the C SDK tests with LLVM source-based coverage" OFF)

add_executable(licensify_c_tests tests/test_licensify_c.c)
target_include_directories(licensify_c_tests PRIVATE
  ${CLIENT_INCLUDE_DIR}
  ${CMAKE_CURRENT_SOURCE_DIR}/include
)
target_link_directories(licensify_c_tests PRIVATE
  ${LIBLICENSIFY_DEBUG_DIR}
  ${LIBLICENSIFY_RELEASE_DIR}
)
target_link_libraries(licensify_c_tests PRIVATE licensify)

if(LICENSIFY_C_COVERAGE)
  if(CMAKE_C_COMPILER_ID MATCHES "Clang")
    target_compile_options(licensify_c_tests PRIVATE -fprofile-instr-generate -fcoverage-mapping -O0 -g)
    target_link_options(licensify_c_tests PRIVATE -fprofile-instr-generate -fcoverage-mapping)
  else()
    target_compile_options(licensify_c_tests PRIVATE --coverage -O0 -g)
    target_link_options(licensify_c_tests PRIVATE --coverage)
  endif()
endif()

# Embed an rpath so the runtime loader can find liblicensify next to the build.
if(APPLE)
  set_target_properties(licensify_c_tests PROPERTIES INSTALL_RPATH "@loader_path")
  target_link_options(licensify_c_tests PRIVATE
    "-Wl,-rpath,${LIBLICENSIFY_DEBUG_DIR}"
    "-Wl,-rpath,${LIBLICENSIFY_RELEASE_DIR}"
  )
elseif(UNIX)
  target_link_options(licensify_c_tests PRIVATE
    "-Wl,-rpath,${LIBLICENSIFY_DEBUG_DIR}"
    "-Wl,-rpath,${LIBLICENSIFY_RELEASE_DIR}"
  )
endif()

enable_testing()
add_test(NAME licensify_c_tests COMMAND licensify_c_tests)
