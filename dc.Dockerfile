FROM docker-hub-remote.dr.corp.adobe.com/alpine:3.14
RUN apk update --no-cache && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
COPY ./cmd/kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]