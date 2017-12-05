FROM docker.io/library/golang:1.9-alpine AS builder
WORKDIR /go/src/github.com/tonistiigi/llb-gobuild
RUN apk add --no-cache git
COPY . .
RUN go get -d github.com/moby/buildkit/client/llb && rm -rf /go/src/github.com/moby/buildkit/vendor/github.com/opencontainers/go-digest && go get -v ./cmd/gobuild
RUN CGO_ENABLED=0 go build -o /out/gobuild --ldflags '-s -w -extldflags "-static"' ./cmd/gobuild

FROM scratch
COPY --from=builder /out/gobuild /bin/gobuild
ENV PATH=/bin
ENTRYPOINT ["/gobuild"]