add_executable(lb_tests
    test_buffer.cpp
    test_address.cpp
    test_round_robin.cpp
    test_least_conn.cpp
    test_weighted.cpp
    test_ip_hash.cpp
    test_health_checker.cpp
    test_config.cpp
    test_proxy_integration.cpp
)

target_link_libraries(lb_tests PRIVATE
    lb_core lb_net lb_balancer lb_proxy lb_metrics GTest::gtest_main)

include(GoogleTest)
gtest_discover_tests(lb_tests)
