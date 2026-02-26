FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o collector ./cmd/collector

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/collector .
COPY data/ ./data/
EXPOSE 33740/udp 8080
ENTRYPOINT ["./collector"]
CMD ["--config", "/app/config.yaml"]
