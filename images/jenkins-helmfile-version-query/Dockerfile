FROM alpine:3.11.5

ARG yq_version=4.11.2
ARG os=linux
ARG arch=amd64

RUN apk add curl jq

# Download yq
https://github.com/mikefarah/yq/releases/download/v4.11.2/yq_linux_amd64.tar.gz
wget https://github.com/mikefarah/yq/releases/download/v${yq_version}/yq_${os}_${arch}.tar.gz -O - |\
  tar xz && \
  mv yq_${os}_${arch} /bin/yq && \
  yq -- version

COPY query.sh /query.sh

ENTRYPOINT ["/query.sh"]
