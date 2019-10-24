FROM alpine:3.8
RUN apk update && apk add ca-certificates tzdata && rm -rf /var/cache/apk/*
# NB: you may need to update RBAC permissions when upgrading kubectl - see kured-rbac.yaml for details
ADD https://storage.googleapis.com/kubernetes-release/release/v1.14.1/bin/linux/amd64/kubectl /usr/bin/kubectl
RUN chmod 0755 /usr/bin/kubectl
COPY ./kured /usr/bin/kured
ENTRYPOINT ["/usr/bin/kured"]
