FROM alpine:3.12
RUN apk add --no-cache ca-certificates tzdata
COPY ./kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
