# syntax=docker/dockerfile:1
# Multi-stage build producing a tiny static image for running regionlock in CI
# or as an admission-adjacent job.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o /out/regionlock ./cmd/regionlock

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/RamazanKara/regionlock"
LABEL org.opencontainers.image.description="Enforce & evidence EU data-residency on Kubernetes"
LABEL org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/regionlock /usr/local/bin/regionlock
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/regionlock"]
CMD ["--help"]
