add_library(server_core STATIC
    protocol.cpp
    session.cpp
)

target_include_directories(server_core PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(server_core PUBLIC matching_engine)

if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    add_executable(matching_server
        main.cpp
        tcp_server.cpp
    )
    target_link_libraries(matching_server PRIVATE server_core)
endif()
