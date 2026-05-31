FROM ubuntu:24.04 AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++-13 make ca-certificates \
    && rm -rf /var/lib/apt/lists/*
ENV CXX=g++-13
WORKDIR /src
COPY . .
RUN cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTS=OFF \
    && cmake --build build --parallel

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /src/build/src/server/load_balancer /app/load_balancer
COPY --from=build /src/build/src/benchmark/lb_benchmark /app/lb_benchmark
COPY config/example.conf /etc/lb/example.conf
EXPOSE 8080 9100
ENTRYPOINT ["/app/load_balancer"]
CMD ["--config", "/etc/lb/example.conf"]
