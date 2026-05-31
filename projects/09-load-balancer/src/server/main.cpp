#include <chrono>
#include <memory>
#include <string>
#include <string_view>
#include <thread>

#include "../balancer/backend_pool.hpp"
#include "../balancer/health_checker.hpp"
#include "../balancer/strategy.hpp"
#include "../core/config.hpp"
#include "../core/logger.hpp"
#include "../core/signal_handler.hpp"
#include "../metrics/collector.hpp"
#include "../metrics/prometheus.hpp"
#include "../net/address.hpp"
#include "../proxy/proxy_engine.hpp"
#include "../reload/config_watcher.hpp"

namespace {

struct Args {
    std::string config_path = "config/example.conf";
    int port_override = 0;
};

Args parse_args(int argc, char** argv) {
    Args args;
    for (int i = 1; i < argc; ++i) {
        const std::string_view arg = argv[i];
        if (arg == "--config" && i + 1 < argc) {
            args.config_path = argv[++i];
        } else if (arg == "--port" && i + 1 < argc) {
            args.port_override = std::atoi(argv[++i]);
        }
    }
    return args;
}

void populate_pool(lb::BackendPool& pool, const lb::Config& cfg) {
    for (const auto& backend : cfg.backends) {
        auto addr = lb::Address::from_string(backend.address);
        if (!addr) {
            lb::Logger::instance().error("skipping invalid backend", {{"address", backend.address}});
            continue;
        }
        pool.add(std::make_shared<lb::Backend>(*addr, backend.address, backend.weight, backend.max_connections));
    }
}

}

int main(int argc, char** argv) {
    const Args args = parse_args(argc, argv);
    lb::Logger& log = lb::Logger::instance();

    std::string error;
    auto cfg = lb::Config::from_file(args.config_path, error);
    if (!cfg) {
        log.error("failed to load config", {{"error", error}});
        return 1;
    }
    if (args.port_override > 0) {
        const std::size_t colon = cfg->listen.rfind(':');
        const std::string host = colon == std::string::npos ? cfg->listen : cfg->listen.substr(0, colon);
        cfg->listen = host + ":" + std::to_string(args.port_override);
    }

    auto listen_addr = lb::Address::from_string(cfg->listen);
    if (!listen_addr) {
        log.error("invalid listen address", {{"listen", cfg->listen}});
        return 1;
    }

    lb::SignalHandler::install();

    lb::BackendPool pool;
    populate_pool(pool, *cfg);

    lb::MetricsCollector metrics;
    auto strategy = lb::make_strategy(cfg->strategy);

    lb::HealthChecker health(pool, cfg->health_check, &metrics);
    health.check_once();
    health.start();

    std::unique_ptr<lb::PrometheusExporter> exporter;
    if (cfg->metrics.enabled) {
        exporter = std::make_unique<lb::PrometheusExporter>(metrics, pool, cfg->metrics.port);
        if (exporter->start()) {
            log.info("metrics endpoint started", {{"port", std::to_string(cfg->metrics.port)}});
        }
    }

    lb::ConfigWatcher watcher(args.config_path, pool, *cfg);

    lb::ProxyEngine engine(*listen_addr, pool, *strategy, metrics);
    if (!engine.start()) {
        return 1;
    }

    log.info("load balancer started", {{"strategy", lb::strategy_name(cfg->strategy)}});

    while (!lb::SignalHandler::should_stop()) {
        engine.run_once(200);
        if (lb::SignalHandler::reload_requested()) {
            lb::SignalHandler::clear_reload();
            log.info("reloading configuration");
            watcher.reload();
        }
    }

    log.info("shutdown requested, draining connections");
    const auto deadline = lb::Clock::now() + cfg->drain_timeout;
    while (engine.active_connections() > 0 && lb::Clock::now() < deadline) {
        engine.run_once(100);
    }

    engine.stop();
    health.stop();
    if (exporter) {
        exporter->stop();
    }
    log.info("load balancer stopped");
    return 0;
}
