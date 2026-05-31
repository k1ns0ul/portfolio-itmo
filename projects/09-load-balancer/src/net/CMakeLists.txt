add_library(lb_net STATIC
    address.cpp
    socket.cpp
    epoll.cpp
    buffer.cpp
)

target_include_directories(lb_net PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(lb_net PUBLIC lb_core)
