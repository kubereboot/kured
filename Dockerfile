FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS bin

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

FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY --from=bin /dist/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
