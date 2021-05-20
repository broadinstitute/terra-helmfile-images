# Container image that runs your code
FROM alpine:3

# These are specified in cloudbuild.yaml
ARG helmfile_version
ARG helm_version

ARG os=linux
ARG arch=amd64

# Add required OS packages
# needed for entrypoint.sh
# bash needed for `render` script
RUN apk --update add --no-cache bash

# Create /tools directory
RUN mkdir -p /tools/bin

# Add /tools/bin/ to PATH
ENV PATH="/tools/bin:${PATH}"

# Helm
RUN wget --timeout=15 -q -O- "https://get.helm.sh/helm-v${helm_version}-${os}-${arch}.tar.gz" | \
    tar -xz --strip-components=1 "${os}-${arch}/helm" && \
    chmod +x helm && \
    mv helm /tools/bin && \
    helm version

# Helmfile
RUN wget --timeout=15 -q -O helmfile "https://github.com/roboll/helmfile/releases/download/v${helmfile_version}/helmfile_${os}_${arch}" && \
    chmod +x helmfile && \
    mv helmfile /tools/bin && \
    helmfile --version

COPY ./entrypoint.sh entrypoint.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["./entrypoint.sh"]
