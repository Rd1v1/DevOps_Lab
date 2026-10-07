# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notes .

FROM builder AS test
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go test ./... -v

FROM builder AS pressure-builder
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/memory-pressure ./cmd/memory-pressure

FROM alpine:3.22 AS runtime
RUN adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=builder /out/notes /app/notes
COPY --chmod=755 healthcheck.sh /app/healthcheck.sh
USER app
EXPOSE 8000
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 CMD ["/app/healthcheck.sh"]
CMD ["/app/notes"]

FROM runtime AS diagnostics
COPY --from=pressure-builder /out/memory-pressure /app/memory-pressure

FROM runtime AS final
