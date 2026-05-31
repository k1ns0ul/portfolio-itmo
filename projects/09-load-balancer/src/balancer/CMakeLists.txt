add_library(lb_balancer STATIC
    backend.cpp
    backend_pool.cpp
    strategy.cpp
    round_robin.cpp
    least_connections.cpp
    weighted.cpp
    ip_hash.cpp
    health_checker.cpp
)

target_include_directories(lb_balancer PUBLIC ${CMAKE_CURRENT_SOURCE_DIR} ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lb_balancer PUBLIC lb_core lb_net)
