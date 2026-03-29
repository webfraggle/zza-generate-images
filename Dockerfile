# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG ZZA_VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X github.com/webfraggle/zza-generate-images/internal/version.Version=${ZZA_VERSION}" -o zza ./cmd/zza

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 zza

WORKDIR /app
COPY --from=builder /app/zza .

RUN mkdir -p /data/cache /data/db /data/templates \
    && chown -R zza:zza /data

USER zza
EXPOSE 8080

ENTRYPOINT ["/app/zza", "serve"]
