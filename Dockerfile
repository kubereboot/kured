FROM alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375 AS bin

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

COPY dist/ /dist
RUN set -ex \
  && case "${TARGETARCH}" in \
      amd64) \
          SUFFIX="_v1" \
          ;; \
      arm) \
          SUFFIX="_${TARGETVARIANT:1}" \
          ;; \
      *) \
          SUFFIX="" \
          ;; \
    esac \
  && cp /dist/kured_${TARGETOS}_${TARGETARCH}${SUFFIX}/kured /dist/kured;

FROM alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
