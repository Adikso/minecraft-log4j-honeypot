# Build environment
FROM golang:1.17-alpine AS builder

COPY . $GOPATH/src/github.com/Adikso/minecraft-log4j-honeypot
WORKDIR $GOPATH/src/github.com/Adikso/minecraft-log4j-honeypot
RUN go install .

# Export binary only from builder environment
FROM alpine:latest AS runner

COPY --from=builder /go/bin/minecraft-log4j-honeypot /usr/local/bin/minecraft-log4j-honeypot

VOLUME payloads
EXPOSE 25565

ENTRYPOINT ["/usr/local/bin/minecraft-log4j-honeypot"]
