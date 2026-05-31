#include "backend_pool.hpp"

#include <algorithm>
#include <mutex>

namespace lb {

void BackendPool::add(BackendPtr backend) {
    std::unique_lock lock(mutex_);
    backends_.push_back(std::move(backend));
}

bool BackendPool::remove(const std::string& label) {
    std::unique_lock lock(mutex_);
    const auto it = std::find_if(backends_.begin(), backends_.end(),
                                 [&label](const BackendPtr& b) { return b->label() == label; });
    if (it == backends_.end()) {
        return false;
    }
    backends_.erase(it);
    return true;
}

std::vector<BackendPtr> BackendPool::snapshot() const {
    std::shared_lock lock(mutex_);
    return backends_;
}

std::vector<BackendPtr> BackendPool::healthy() const {
    std::shared_lock lock(mutex_);
    std::vector<BackendPtr> result;
    result.reserve(backends_.size());
    for (const auto& backend : backends_) {
        if (backend->healthy() && !backend->draining) {
            result.push_back(backend);
        }
    }
    return result;
}

BackendPtr BackendPool::find(const std::string& label) const {
    std::shared_lock lock(mutex_);
    const auto it = std::find_if(backends_.begin(), backends_.end(),
                                 [&label](const BackendPtr& b) { return b->label() == label; });
    return it == backends_.end() ? nullptr : *it;
}

void BackendPool::mark_down(const std::string& label) {
    std::shared_lock lock(mutex_);
    for (const auto& backend : backends_) {
        if (backend->label() == label) {
            backend->set_healthy(false);
        }
    }
}

void BackendPool::mark_up(const std::string& label) {
    std::shared_lock lock(mutex_);
    for (const auto& backend : backends_) {
        if (backend->label() == label) {
            backend->set_healthy(true);
        }
    }
}

std::size_t BackendPool::size() const {
    std::shared_lock lock(mutex_);
    return backends_.size();
}

}
