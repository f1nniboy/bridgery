FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -o bridgery ./cmd/bridgery

FROM gcr.io/distroless/static-debian12

LABEL org.opencontainers.image.source="https://github.com/f1nniboy/bridgery"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=build /app/bridgery /bridgery
USER nonroot:nonroot
ENTRYPOINT ["/bridgery", "-config", "/etc/bridgery/config.toml"]
