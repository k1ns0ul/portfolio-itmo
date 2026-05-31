add_library(lb_reload STATIC
    config_watcher.cpp
)

target_include_directories(lb_reload PUBLIC ${CMAKE_CURRENT_SOURCE_DIR} ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lb_reload PUBLIC lb_core lb_net lb_balancer)
