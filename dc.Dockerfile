ARG GO_VERSION=1.17
FROM docker-hub-remote.dr.corp.adobe.com/golang:${GO_VERSION}-alpine AS builder
WORKDIR /build
COPY go.* /build/
RUN go mod download
COPY . /build
WORKDIR /build/cmd/kured
ENV CGO_ENABLED=0
RUN go build -v

FROM docker-hub-remote.dr.corp.adobe.com/alpine:3.14
COPY --from=builder /build/cmd/kured /
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ./cmd/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
