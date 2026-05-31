#include "config_watcher.hpp"

#include <algorithm>
#include <unordered_set>
#include <utility>

#include "../core/logger.hpp"
#include "../net/address.hpp"

namespace lb {

ConfigWatcher::ConfigWatcher(std::string path, BackendPool& pool, Config current)
    : path_(std::move(path)), pool_(pool), current_(std::move(current)) {}

std::string ConfigWatcher::label_for(const BackendConfig& backend) {
    return backend.address;
}

bool ConfigWatcher::reload() {
    std::string error;
    auto fresh = Config::from_file(path_, error);
    if (!fresh) {
        Logger::instance().error("config reload failed", {{"error", error}});
        return false;
    }

    if (fresh->listen != current_.listen) {
        Logger::instance().error("listen address change requires restart",
                                 {{"old", current_.listen}, {"new", fresh->listen}});
    }
    if (fresh->strategy != current_.strategy) {
        Logger::instance().info("strategy change requires component reload",
                                {{"old", strategy_name(current_.strategy)},
                                 {"new", strategy_name(fresh->strategy)}});
    }

    apply(*fresh);
    current_ = std::move(*fresh);
    return true;
}

void ConfigWatcher::apply(const Config& fresh) {
    std::unordered_set<std::string> desired;
    for (const auto& backend : fresh.backends) {
        desired.insert(label_for(backend));
    }

    for (const auto& existing : pool_.snapshot()) {
        if (desired.find(existing->label()) == desired.end()) {
            existing->draining = true;
            Logger::instance().info("backend draining for removal", {{"backend", existing->label()}});
            if (existing->active_connections() == 0) {
                pool_.remove(existing->label());
                Logger::instance().info("backend removed", {{"backend", existing->label()}});
            }
        }
    }

    for (const auto& backend : fresh.backends) {
        const std::string label = label_for(backend);
        if (auto current = pool_.find(label)) {
            if (current->weight() != backend.weight) {
                current->set_weight(backend.weight);
                Logger::instance().info("backend weight updated",
                                        {{"backend", label}, {"weight", field_int(backend.weight)}});
            }
            current->draining = false;
            continue;
        }
        auto addr = Address::from_string(backend.address);
        if (!addr) {
            Logger::instance().error("invalid backend address in reload", {{"address", backend.address}});
            continue;
        }
        auto created = std::make_shared<Backend>(*addr, label, backend.weight, backend.max_connections);
        pool_.add(created);
        Logger::instance().info("backend added", {{"backend", label}});
    }
}

}
