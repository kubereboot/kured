FROM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

ARG TARGETPLATFORM

RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ${TARGETPLATFORM}/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
