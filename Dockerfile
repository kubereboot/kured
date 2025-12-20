FROM alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 AS bin

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN apk update --no-cache && apk add --no-cache jq

COPY dist/ /dist

# Fetch binary directory from artifacts.json
RUN set -ex \
  && case "${TARGETARCH}" in \
      arm) \
          BINARY_PATH=$(jq -r 'first(.[] | select(.goos == "linux" and .type == "Binary" and .goarch == "arm" and .goarm == env.TARGETVARIANT[1:] ) | .path)' /dist/artifacts.json) \
          ;; \
      *) \
          BINARY_PATH=$(jq -r 'first(.[] | select(.goos == "linux" and .type == "Binary" and .goarch == env.TARGETARCH) | .path)' /dist/artifacts.json) \
          ;; \
    esac \
  && cp /${BINARY_PATH} /dist/kured;

FROM alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
