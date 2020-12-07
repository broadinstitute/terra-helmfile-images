FROM alpine:3.11.5

ARG os=linux
ARG arch=amd64

RUN apk add curl jq

# TODO - this is only necessary until yq is part of the regular alpine distro
# https://github.com/mikefarah/yq/issues/190
RUN apk add yq --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community

COPY query.sh /query.sh

ENTRYPOINT ["/query"]
