FROM --platform=$TARGETPLATFORM alpine:3.16.2 as bin

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

FROM --platform=$TARGETPLATFORM alpine:3.16.2
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
