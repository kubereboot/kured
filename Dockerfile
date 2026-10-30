FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

ARG TARGETPLATFORM

RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ${TARGETPLATFORM}/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
