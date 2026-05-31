#pragma once

#include <cstdint>
#include <initializer_list>
#include <mutex>
#include <string>
#include <string_view>
#include <utility>

namespace lb {

enum class LogLevel : std::uint8_t { Debug, Info, Warn, Error };

using LogField = std::pair<std::string_view, std::string>;

class Logger {
public:
    static Logger& instance();

    void set_level(LogLevel level) noexcept { level_ = level; }

    void log(LogLevel level, std::string_view message, std::initializer_list<LogField> fields = {});

    void debug(std::string_view message, std::initializer_list<LogField> fields = {}) {
        log(LogLevel::Debug, message, fields);
    }
    void info(std::string_view message, std::initializer_list<LogField> fields = {}) {
        log(LogLevel::Info, message, fields);
    }
    void warn(std::string_view message, std::initializer_list<LogField> fields = {}) {
        log(LogLevel::Warn, message, fields);
    }
    void error(std::string_view message, std::initializer_list<LogField> fields = {}) {
        log(LogLevel::Error, message, fields);
    }

private:
    Logger() = default;

    LogLevel level_ = LogLevel::Info;
    std::mutex mutex_;
};

std::string field_int(long value);

}
