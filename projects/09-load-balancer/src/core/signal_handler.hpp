#pragma once

#include <atomic>
#include <csignal>

namespace lb {

class SignalHandler {
public:
    static void install();

    static bool should_stop() noexcept { return stop_requested_.load(std::memory_order_acquire); }
    static bool reload_requested() noexcept { return reload_flag_.load(std::memory_order_acquire); }
    static void clear_reload() noexcept { reload_flag_.store(false, std::memory_order_release); }

private:
    static void on_signal(int sig);

    static std::atomic<bool> stop_requested_;
    static std::atomic<bool> reload_flag_;
};

}
