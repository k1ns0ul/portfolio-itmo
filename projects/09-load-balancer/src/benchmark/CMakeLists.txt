find_package(Threads REQUIRED)

add_executable(lb_benchmark main.cpp)
target_link_libraries(lb_benchmark PRIVATE
    lb_core lb_net lb_balancer lb_proxy lb_metrics Threads::Threads)
