# syntax=docker/dockerfile:1.8
ARG GO_VERSION=1.25
ARG BUF_VERSION=1.66.0

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS buf
ARG BUF_VERSION
RUN apk add --no-cache curl
RUN curl -sSL \
      "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(uname -s)-$(uname -m)" \
      -o /usr/local/bin/buf && \
    chmod +x /usr/local/bin/buf

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf

ARG BPE_URL=https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken
RUN apk add --no-cache curl

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY buf.gen.yaml buf.yaml ./
RUN buf generate buf.build/agynio/api --path agynio/api/token_counting/v1

COPY . .

RUN mkdir -p /out/encodings && curl -sSL "$BPE_URL" -o /out/encodings/o200k_base.tiktoken

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /out/token-counting ./cmd/token-counting

FROM alpine:3.21 AS runtime

WORKDIR /app

COPY --from=build /out/token-counting /app/token-counting
COPY --from=build /out/encodings/o200k_base.tiktoken /app/encodings/o200k_base.tiktoken

ENV TOKEN_COUNTING_BPE_PATH=/app/encodings/o200k_base.tiktoken

RUN addgroup -S app && adduser -S app -G app

USER app

ENTRYPOINT ["/app/token-counting"]
