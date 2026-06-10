FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

ARG TARGETPLATFORM

RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ${TARGETPLATFORM}/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
