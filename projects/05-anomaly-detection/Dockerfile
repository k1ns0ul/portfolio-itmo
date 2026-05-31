# syntax=docker/dockerfile:1.7
ARG TARGET=api

FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG TARGET
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w" \
    -o /out/service ./cmd/${TARGET}

FROM gcr.io/distroless/static:nonroot AS runtime
WORKDIR /app
COPY --from=builder /out/service /app/service
USER nonroot:nonroot
ENTRYPOINT ["/app/service"]
