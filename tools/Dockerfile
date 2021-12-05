#
# Compile Go tools and install to /tools/bin
#
ARG GO_VERSION='1.16'
ARG ALPINE_VERSION='3.14'

#
# Build & test code
#
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} as build

WORKDIR /build
ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOBIN=/bin/
COPY . .
RUN go test ./... && go build -o /tools/bin/ ./cmd/...

#
# Download runtime dependencies (Helm, Helmfile) and install to /tools/bin
#
FROM alpine:${ALPINE_VERSION} as runtime-deps

ARG HELM_VERSION=3.2.4
ARG HELMFILE_VERSION=0.114.0
ARG YQ_VERSION=4.11.2
ARG HELM_DOCS_VERSION=1.5.0
ARG OS=linux
ARG ARCH=amd64

RUN mkdir -p /tools/bin

# Install Helm
RUN wget --timeout=15 -q -O- "https://get.helm.sh/helm-v${HELM_VERSION}-${OS}-${ARCH}.tar.gz" | \
    tar -xz --strip-components=1 "${OS}-${ARCH}/helm" && \
    chmod +x helm && \
    mv helm /tools/bin && \
    /tools/bin/helm version

# Install Helmfile
RUN wget --timeout=15 -q -O helmfile "https://github.com/roboll/helmfile/releases/download/v${HELMFILE_VERSION}/helmfile_${OS}_${ARCH}" && \
    chmod +x helmfile && \
    mv helmfile /tools/bin && \
    /tools/bin/helmfile --version

# Install yq
RUN wget https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_${OS}_${ARCH}.tar.gz -O - |\
  tar xz && \
  mv yq_${OS}_${ARCH} /tools/bin/yq && \
  /tools/bin/yq --version

# Install helm-docs
# amd64 is only made available as an rpm or deb package and not a tarball, so we install RPM to s
RUN apk add rpm && \
    wget https://github.com/norwoodj/helm-docs/releases/download/v${HELM_DOCS_VERSION}/helm-docs_${HELM_DOCS_VERSION}_${OS}_${ARCH}.rpm -O helm-docs.rpm && \
    rpm -i helm-docs.rpm --ignoresize && \
    mv /usr/local/bin/helm-docs /tools/bin && \
    /tools/bin/helm-docs --version

#
# Copy tools into runtime image
#
FROM alpine:${ALPINE_VERSION} as runtime

# OS updates for security
RUN apk update
RUN apk upgrade

# Copy tools into runtime image
COPY --from=build /tools/bin/ /tools/bin/
COPY --from=runtime-deps /tools/bin/ /tools/bin/
ENV PATH="/tools/bin:${PATH}"
