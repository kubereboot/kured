FROM alpine:3.15.0

ARG TARGETOS
ARG TARGETARCH

RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata

COPY ./kured_${TARGETOS}_${TARGETARCH} /usr/bin/kured

ENTRYPOINT ["/usr/bin/kured"]
