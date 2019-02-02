# syntax = docker/dockerfile:experimental

FROM docker.io/library/golang:1.11-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
RUN --mount=target=. \
	--mount=target=/go/pkg/mod,type=cache \
	CGO_ENABLED=0 go build -o /out/gobuild --ldflags '-s -w -extldflags "-static"' ./cmd/gobuild

FROM scratch
COPY --from=builder /out/gobuild /bin/gobuild
ENV PATH=/bin
ENTRYPOINT ["/gobuild"]