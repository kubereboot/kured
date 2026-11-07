FROM alpine:3.24.4@sha256:93b41ef40b1677fce8fcc64ff4af09daddfacf5040afde618bc46c1e2d1cc7d4

ARG TARGETPLATFORM

RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ${TARGETPLATFORM}/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
