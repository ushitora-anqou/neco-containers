# Go container

FROM --platform=$TARGETPLATFORM ghcr.io/cybozu/ubuntu-dev:22.04

ARG TARGETARCH
ARG GO_VERSION=1.20.4

ENV GOARCH=${TARGETARCH}
ENV GOPATH=/go
ENV PATH=/go/bin:/usr/local/go/bin:"$PATH"

RUN rm -rf /usr/local/go \
    && curl -sfL https://dl.google.com/go/go${GO_VERSION}.linux-${GOARCH}.tar.gz \
    | tar -x -z -C /usr/local -f - \
    && mkdir -p /go/src \
    && GOBIN=/usr/local/bin go install golang.org/x/tools/cmd/goimports@latest \
    && GOBIN=/usr/local/bin go install golang.org/x/lint/golint@latest \
    && GOBIN=/usr/local/bin go install honnef.co/go/tools/cmd/staticcheck@latest \
    && GOBIN=/usr/local/bin go install github.com/gordonklaus/ineffassign@latest \
    && GOBIN=/usr/local/bin go install github.com/tcnksm/ghr@latest \
    && rm -rf /go \
    && mkdir -p /go/src

COPY ./analyzer /work/analyzer
WORKDIR /work/analyzer
RUN go mod tidy && GOBIN=/usr/local/bin go install ./cmd/...

WORKDIR /work
RUN rm -r /work/analyzer

CMD ["/bin/bash"]
