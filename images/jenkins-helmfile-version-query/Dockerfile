FROM alpine:3

ARG yq_version=4.11.2
ARG os=linux
ARG arch=amd64

RUN apk add curl jq

# Download yq
RUN wget https://github.com/mikefarah/yq/releases/download/v${yq_version}/yq_${os}_${arch}.tar.gz -O - |\
  tar xz && \
  mv yq_${os}_${arch} /bin/yq && \
  yq -- version

COPY query.sh /query.sh

ENTRYPOINT ["/query.sh"]
