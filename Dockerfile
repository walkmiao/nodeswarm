# Multi-architecture image. Linux host networking is recommended for the
# cloud Agent's SOCKS5 TCP and bounded UDP relay ports.
FROM --platform=$BUILDPLATFORM golang:1.23.2-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /node-console .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates && addgroup -S nodeconsole && adduser -S -G nodeconsole nodeconsole && mkdir -p /data && chown nodeconsole:nodeconsole /data
COPY --from=builder /node-console /usr/local/bin/node-console
USER nodeconsole
VOLUME ["/data"]
ENV NODE_MANAGER_DATA=/data NODE_MANAGER_AGENT_DATA=/data/agent
ENTRYPOINT ["/usr/local/bin/node-console"]
CMD ["-mode", "controller"]
