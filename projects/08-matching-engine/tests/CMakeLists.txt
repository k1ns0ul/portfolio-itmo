add_executable(matching_tests
    test_order_book.cpp
    test_matching.cpp
    test_protocol.cpp
)

target_link_libraries(matching_tests PRIVATE matching_engine server_core GTest::gtest_main)

include(GoogleTest)
gtest_discover_tests(matching_tests)
