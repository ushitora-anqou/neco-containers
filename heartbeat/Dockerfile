FROM quay.io/cybozu/golang:1.20-jammy as build

COPY / /work
WORKDIR /work

RUN go build -ldflags="-w -s" .

# stage2: final image
FROM quay.io/cybozu/ubuntu:22.04

COPY --from=build /work/heartbeat /usr/local/bin/heartbeat
USER 10000:10000

ENTRYPOINT ["/usr/local/bin/heartbeat"]
