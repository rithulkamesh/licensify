# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Rithul Kamesh
# Author: Rithul Kamesh <hi@rithul.dev>
# Description: sdk/cpp/CMakeLists.txt — Header-only C++ SDK + Catch2-based coverage tests.

cmake_minimum_required(VERSION 3.20)
project(licensify_sdk_cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CLIENT_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../client/include")
set(LIBLICENSIFY_DEBUG_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../target/debug")
set(LIBLICENSIFY_RELEASE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../target/release")

add_library(licensify_sdk_cpp INTERFACE)
target_include_directories(licensify_sdk_cpp INTERFACE
  ${CMAKE_CURRENT_SOURCE_DIR}/include
  ${CLIENT_INCLUDE_DIR}
)

option(LICENSIFY_CPP_TESTS "Build the Licensify C++ SDK tests" ON)
option(LICENSIFY_CPP_COVERAGE "Build C++ SDK tests with coverage instrumentation" OFF)

if(LICENSIFY_CPP_TESTS)
  include(FetchContent)
  FetchContent_Declare(
    Catch2
    GIT_REPOSITORY https://github.com/catchorg/Catch2.git
    GIT_TAG v3.5.4
  )
  FetchContent_MakeAvailable(Catch2)

  add_executable(licensify_cpp_tests tests/test_client.cpp)
  target_link_libraries(licensify_cpp_tests PRIVATE
    licensify_sdk_cpp
    Catch2::Catch2WithMain
  )
  target_link_directories(licensify_cpp_tests PRIVATE
    ${LIBLICENSIFY_DEBUG_DIR}
    ${LIBLICENSIFY_RELEASE_DIR}
  )
  target_link_libraries(licensify_cpp_tests PRIVATE licensify)

  if(APPLE)
    target_link_options(licensify_cpp_tests PRIVATE
      "-Wl,-rpath,${LIBLICENSIFY_DEBUG_DIR}"
      "-Wl,-rpath,${LIBLICENSIFY_RELEASE_DIR}"
    )
  elseif(UNIX)
    target_link_options(licensify_cpp_tests PRIVATE
      "-Wl,-rpath,${LIBLICENSIFY_DEBUG_DIR}"
      "-Wl,-rpath,${LIBLICENSIFY_RELEASE_DIR}"
    )
  endif()

  if(LICENSIFY_CPP_COVERAGE)
    if(CMAKE_CXX_COMPILER_ID MATCHES "Clang")
      target_compile_options(licensify_cpp_tests PRIVATE -fprofile-instr-generate -fcoverage-mapping -O0 -g)
      target_link_options(licensify_cpp_tests PRIVATE -fprofile-instr-generate -fcoverage-mapping)
    else()
      target_compile_options(licensify_cpp_tests PRIVATE --coverage -O0 -g)
      target_link_options(licensify_cpp_tests PRIVATE --coverage)
    endif()
  endif()

  enable_testing()
  add_test(NAME licensify_cpp_tests COMMAND licensify_cpp_tests)
endif()
