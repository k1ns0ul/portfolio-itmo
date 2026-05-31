#pragma once

#include <string>

#include "../balancer/backend_pool.hpp"
#include "../core/config.hpp"

namespace lb {

class ConfigWatcher {
public:
    ConfigWatcher(std::string path, BackendPool& pool, Config current);

    bool reload();

    const Config& current() const noexcept { return current_; }

private:
    void apply(const Config& fresh);
    static std::string label_for(const BackendConfig& backend);

    std::string path_;
    BackendPool& pool_;
    Config current_;
};

}
