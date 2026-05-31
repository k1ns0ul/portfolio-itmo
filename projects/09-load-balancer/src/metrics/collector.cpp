#include "collector.hpp"

#include <memory>
#include <sstream>

namespace lb {

constexpr std::array<double, MetricsCollector::kBucketCount - 1> MetricsCollector::kBucketBounds;

MetricsCollector::BackendMetrics& MetricsCollector::backend_metrics(const std::string& backend) {
    std::lock_guard<std::mutex> guard(backends_mutex_);
    auto it = backends_.find(backend);
    if (it == backends_.end()) {
        it = backends_.emplace(backend, std::make_unique<BackendMetrics>()).first;
    }
    return *it->second;
}

void MetricsCollector::on_connection_open() {
    total_connections_.fetch_add(1, std::memory_order_relaxed);
    active_connections_.fetch_add(1, std::memory_order_relaxed);
}

void MetricsCollector::on_connection_close(double duration_seconds) {
    active_connections_.fetch_sub(1, std::memory_order_relaxed);
    std::size_t bucket = kBucketCount - 1;
    for (std::size_t i = 0; i < kBucketBounds.size(); ++i) {
        if (duration_seconds <= kBucketBounds[i]) {
            bucket = i;
            break;
        }
    }
    duration_buckets_[bucket].fetch_add(1, std::memory_order_relaxed);
}

void MetricsCollector::on_failed_connection() {
    failed_connections_.fetch_add(1, std::memory_order_relaxed);
}

void MetricsCollector::add_bytes_in(const std::string& backend, std::uint64_t n) {
    total_bytes_in_.fetch_add(n, std::memory_order_relaxed);
    backend_metrics(backend).bytes_in.fetch_add(n, std::memory_order_relaxed);
}

void MetricsCollector::add_bytes_out(const std::string& backend, std::uint64_t n) {
    total_bytes_out_.fetch_add(n, std::memory_order_relaxed);
    backend_metrics(backend).bytes_out.fetch_add(n, std::memory_order_relaxed);
}

void MetricsCollector::on_backend_connection(const std::string& backend) {
    backend_metrics(backend).connections.fetch_add(1, std::memory_order_relaxed);
}

void MetricsCollector::set_backend_health(const std::string& backend, bool healthy) {
    backend_metrics(backend).healthy.store(healthy ? 1 : 0, std::memory_order_relaxed);
}

void MetricsCollector::set_backend_response_time(const std::string& backend, std::int64_t us) {
    backend_metrics(backend).response_time_us.store(us, std::memory_order_relaxed);
}

void MetricsCollector::record_health_check(const std::string& backend, bool success) {
    health_checks_total_.fetch_add(1, std::memory_order_relaxed);
    BackendMetrics& m = backend_metrics(backend);
    if (success) {
        m.health_success.fetch_add(1, std::memory_order_relaxed);
    } else {
        m.health_failure.fetch_add(1, std::memory_order_relaxed);
        health_checks_failed_.fetch_add(1, std::memory_order_relaxed);
    }
}

std::string MetricsCollector::render_prometheus() const {
    std::ostringstream out;

    out << "# HELP lb_connections_total Total accepted connections per backend\n";
    out << "# TYPE lb_connections_total counter\n";
    {
        std::lock_guard<std::mutex> guard(backends_mutex_);
        for (const auto& [label, m] : backends_) {
            out << "lb_connections_total{backend=\"" << label << "\"} " << m->connections.load() << "\n";
        }

        out << "# HELP lb_connections_active Active connections\n";
        out << "# TYPE lb_connections_active gauge\n";
        out << "lb_connections_active " << active_connections_.load() << "\n";

        out << "# HELP lb_bytes_total Bytes proxied per direction and backend\n";
        out << "# TYPE lb_bytes_total counter\n";
        for (const auto& [label, m] : backends_) {
            out << "lb_bytes_total{direction=\"in\",backend=\"" << label << "\"} " << m->bytes_in.load() << "\n";
            out << "lb_bytes_total{direction=\"out\",backend=\"" << label << "\"} " << m->bytes_out.load() << "\n";
        }

        out << "# HELP lb_backend_healthy Backend health state\n";
        out << "# TYPE lb_backend_healthy gauge\n";
        for (const auto& [label, m] : backends_) {
            out << "lb_backend_healthy{backend=\"" << label << "\"} " << m->healthy.load() << "\n";
        }

        out << "# HELP lb_backend_response_time_microseconds Last health check response time\n";
        out << "# TYPE lb_backend_response_time_microseconds gauge\n";
        for (const auto& [label, m] : backends_) {
            out << "lb_backend_response_time_microseconds{backend=\"" << label << "\"} "
                << m->response_time_us.load() << "\n";
        }

        out << "# HELP lb_health_checks_total Health checks by result\n";
        out << "# TYPE lb_health_checks_total counter\n";
        for (const auto& [label, m] : backends_) {
            out << "lb_health_checks_total{backend=\"" << label << "\",result=\"success\"} "
                << m->health_success.load() << "\n";
            out << "lb_health_checks_total{backend=\"" << label << "\",result=\"failure\"} "
                << m->health_failure.load() << "\n";
        }
    }

    out << "# HELP lb_connection_duration_seconds Connection duration histogram\n";
    out << "# TYPE lb_connection_duration_seconds histogram\n";
    std::uint64_t cumulative = 0;
    for (std::size_t i = 0; i < kBucketBounds.size(); ++i) {
        cumulative += duration_buckets_[i].load();
        out << "lb_connection_duration_seconds_bucket{le=\"" << kBucketBounds[i] << "\"} " << cumulative << "\n";
    }
    cumulative += duration_buckets_[kBucketCount - 1].load();
    out << "lb_connection_duration_seconds_bucket{le=\"+Inf\"} " << cumulative << "\n";

    out << "# HELP lb_failed_connections_total Failed upstream connections\n";
    out << "# TYPE lb_failed_connections_total counter\n";
    out << "lb_failed_connections_total " << failed_connections_.load() << "\n";

    return out.str();
}

}
