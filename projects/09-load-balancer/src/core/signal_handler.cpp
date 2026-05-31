#include "signal_handler.hpp"

#include <cstring>

namespace lb {

std::atomic<bool> SignalHandler::stop_requested_{false};
std::atomic<bool> SignalHandler::reload_flag_{false};

void SignalHandler::on_signal(int sig) {
    if (sig == SIGINT || sig == SIGTERM) {
        stop_requested_.store(true, std::memory_order_release);
    } else if (sig == SIGHUP) {
        reload_flag_.store(true, std::memory_order_release);
    }
}

void SignalHandler::install() {
    struct sigaction action{};
    std::memset(&action, 0, sizeof(action));
    action.sa_handler = &SignalHandler::on_signal;
    sigemptyset(&action.sa_mask);
    action.sa_flags = 0;

    sigaction(SIGINT, &action, nullptr);
    sigaction(SIGTERM, &action, nullptr);
    sigaction(SIGHUP, &action, nullptr);

    struct sigaction ignore{};
    ignore.sa_handler = SIG_IGN;
    sigemptyset(&ignore.sa_mask);
    sigaction(SIGPIPE, &ignore, nullptr);
}

}
