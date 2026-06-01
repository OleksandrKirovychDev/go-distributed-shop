# Multi-stage, non-root (distroless) image. Build context is the repo root: the
# module's replace directives point at ../../pkg and ../../proto/gen/go, which must
# be present in the context. Built and run by docker-compose (profile `services`).
#   docker build -f services/identity/Dockerfile -t identity .
FROM golang:1.26-alpine AS build
ENV GOWORK=off
WORKDIR /src

COPY pkg/ pkg/
COPY proto/ proto/
COPY services/identity/ services/identity/

WORKDIR /src/services/identity
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -trimpath -o /out/identity ./cmd/identity

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/identity /identity
USER nonroot:nonroot
EXPOSE 50051 8081
ENTRYPOINT ["/identity"]
