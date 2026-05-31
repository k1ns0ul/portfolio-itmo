FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY go.mod ./
COPY go.su[m] ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kv-node ./cmd/kv-node

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/kv-node /app/kv-node
USER nonroot:nonroot
EXPOSE 8080 7000 7100
ENTRYPOINT ["/app/kv-node"]
