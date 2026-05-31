add_library(lb_core STATIC
    config.cpp
    logger.cpp
    signal_handler.cpp
)

target_include_directories(lb_core PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(lb_core PUBLIC cxx_std_20)
