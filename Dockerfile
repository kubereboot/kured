FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 AS bin

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

FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
