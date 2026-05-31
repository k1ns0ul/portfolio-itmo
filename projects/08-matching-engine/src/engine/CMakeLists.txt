add_library(matching_engine STATIC
    order.cpp
    price_level.cpp
    order_book.cpp
    matching_engine.cpp
)

target_include_directories(matching_engine PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(matching_engine PUBLIC cxx_std_20)

find_package(Threads REQUIRED)
target_link_libraries(matching_engine PUBLIC Threads::Threads)
