add_library(lb_proxy STATIC
    connection.cpp
    proxy_engine.cpp
)

target_include_directories(lb_proxy PUBLIC ${CMAKE_CURRENT_SOURCE_DIR} ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lb_proxy PUBLIC lb_core lb_net lb_balancer lb_metrics)
