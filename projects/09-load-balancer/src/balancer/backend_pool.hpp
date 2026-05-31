#pragma once

#include <memory>
#include <shared_mutex>
#include <string>
#include <vector>

#include "backend.hpp"

namespace lb {

class BackendPool {
public:
    BackendPool() = default;

    void add(BackendPtr backend);
    bool remove(const std::string& label);

    std::vector<BackendPtr> snapshot() const;
    std::vector<BackendPtr> healthy() const;
    BackendPtr find(const std::string& label) const;

    void mark_down(const std::string& label);
    void mark_up(const std::string& label);

    std::size_t size() const;

private:
    mutable std::shared_mutex mutex_;
    std::vector<BackendPtr> backends_;
};

}
