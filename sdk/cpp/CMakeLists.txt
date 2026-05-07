# SPDX-License-Identifier: MIT
cmake_minimum_required(VERSION 3.16)
project(licensify_sdk_cpp LANGUAGES CXX)
add_library(licensify_sdk_cpp INTERFACE)
target_include_directories(licensify_sdk_cpp INTERFACE include ../../client/include)
