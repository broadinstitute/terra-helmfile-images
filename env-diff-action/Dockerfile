# Container image that runs your code
FROM python:3-alpine

# These are specified in cloudbuild.yaml
ARG helmfile_version
ARG helm_version

ARG os=linux
ARG arch=amd64

# Add required OS packages
RUN apk --update add --no-cache bash jq

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

# Python dependencies
COPY requirements.txt /tools/requirements.txt
RUN pip3 install -r /tools/requirements.txt

COPY env-differ   /tools/bin/env-differ
COPY pull-request /tools/bin/pull-request
COPY action.sh /action.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/action.sh"]
