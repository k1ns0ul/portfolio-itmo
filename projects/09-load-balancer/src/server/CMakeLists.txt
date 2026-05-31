find_package(Threads REQUIRED)

add_executable(load_balancer main.cpp)
target_link_libraries(load_balancer PRIVATE
    lb_core lb_net lb_balancer lb_proxy lb_metrics lb_reload Threads::Threads)
