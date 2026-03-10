# syntax=docker/dockerfile:1.8

FROM golang:1.22 AS builder

ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOBIN=/usr/local/bin

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /out/token-counting ./cmd/token-counting

FROM gcr.io/distroless/static-debian12

COPY --from=builder /out/token-counting /bin/token-counting

ENTRYPOINT ["/bin/token-counting"]
