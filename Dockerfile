FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o zza ./cmd/zza

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 zza
WORKDIR /app
COPY --from=builder /app/zza .

USER zza
EXPOSE 8080

ENTRYPOINT ["./zza", "serve"]
