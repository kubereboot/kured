FROM alpine:3.20.3@sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a AS bin

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

FROM alpine:3.20.3@sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
