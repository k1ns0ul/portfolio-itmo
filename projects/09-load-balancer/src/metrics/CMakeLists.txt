add_library(lb_metrics STATIC
    collector.cpp
    prometheus.cpp
)

target_include_directories(lb_metrics PUBLIC ${CMAKE_CURRENT_SOURCE_DIR} ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lb_metrics PUBLIC lb_core lb_net lb_balancer)
