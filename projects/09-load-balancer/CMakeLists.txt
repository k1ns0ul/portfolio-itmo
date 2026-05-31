cmake_minimum_required(VERSION 3.20)
project(load_balancer CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()
set(CMAKE_CXX_FLAGS_RELEASE "-O2")

option(BUILD_TESTS "Build tests" ON)
option(BUILD_BENCHMARK "Build benchmark" ON)

add_subdirectory(src/core)
add_subdirectory(src/net)
add_subdirectory(src/balancer)
add_subdirectory(src/metrics)
add_subdirectory(src/proxy)
add_subdirectory(src/reload)
add_subdirectory(src/server)

if(BUILD_BENCHMARK)
    add_subdirectory(src/benchmark)
endif()

if(BUILD_TESTS)
    enable_testing()
    include(FetchContent)
    FetchContent_Declare(
        googletest
        URL https://github.com/google/googletest/archive/refs/tags/v1.14.0.zip
    )
    set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
    FetchContent_MakeAvailable(googletest)
    add_subdirectory(tests)
endif()
