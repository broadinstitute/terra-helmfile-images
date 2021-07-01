# Specified in cloudbuild.yaml
ARG argocd_version

FROM argoproj/argocd:v${argocd_version}

ARG argocd_version
ARG helmfile_version
ARG vault_version

ARG os=linux
ARG arch=amd64
ARG kustomize_version=3.5.5
ARG consul_template_version=0.20.0

RUN echo "ArgoCD version: ${argocd_version}"

# Switch to root for the ability to perform install
USER root

# Ruby and curl are needed for configure.rb / `legacy-configs` plugin
RUN apt-get update && \
    apt-get install -y \
        wget \
        curl \
        jq \
        ruby \
        ruby-json && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Create tools directory
RUN mkdir -p /custom-tools/bin && chown -R argocd:argocd /custom-tools

ENV PATH="/custom-tools/bin:${PATH}"

# Switch back to non-root user
USER argocd

## Install dependencies for the `legacy-configs` plugin
# (aka the firecloud-develop ctmpl rendering process)

# Kustomize
RUN wget "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${kustomize_version}/kustomize_v${kustomize_version}_${os}_${arch}.tar.gz" && \
    tar -xzf kustomize*.tar.gz && \
    rm -f kustomize*.tar.gz && \
    mv kustomize /custom-tools/bin && \
    kustomize version

# vault
RUN wget "https://releases.hashicorp.com/vault/${vault_version}/vault_${vault_version}_${os}_${arch}.zip"  && \
    unzip vault_${vault_version}_${os}_${arch}.zip && \
    rm -f vault_${vault_version}_${os}_${arch}.zip && \
    chmod +x vault && \
    mv vault /custom-tools/bin && \
    vault -version

# consul-template
RUN wget -O- "https://releases.hashicorp.com/consul-template/${consul_template_version}/consul-template_${consul_template_version}_${os}_${arch}.tgz" | tar -xz && \
    chmod +x consul-template && \
    mv consul-template /custom-tools/bin && \
    consul-template -version

# Cloud SDK (configure.rb depends on `gsutil`)
RUN curl https://sdk.cloud.google.com | bash
ENV PATH="/home/argocd/google-cloud-sdk/bin/:${PATH}"

## Install dependencies for the `helmfile` plugin
# helmfile
RUN wget -O /custom-tools/bin/helmfile https://github.com/roboll/helmfile/releases/download/v${helmfile_version}/helmfile_${os}_${arch} && \
    chmod +x /custom-tools/bin/helmfile && \
    helmfile -version

## `legacy-configs` plugin
COPY legacy-configs /custom-tools/legacy-configs
ENV PATH="/custom-tools/legacy-configs:${PATH}"

## `helm-values` plugin
# Wrapper script for `helm template -f path/to/values.yml`
COPY helm-values /custom-tools/helm-values
ENV PATH="/custom-tools/helm-values:${PATH}"

## `helmfile` plugin
# Wrapper script for `helmfile template`
COPY helmfile /custom-tools/helmfile
ENV PATH="/custom-tools/helmfile:${PATH}"

## `terra-helmfile-argocd` plugin
# Wrapper script for generating ArgoCD resources from terra-helmfile repo
COPY terra-helmfile-argocd /custom-tools/terra-helmfile-argocd
ENV PATH="/custom-tools/terra-helmfile-argocd:${PATH}"

# Copy various config files
COPY etc /etc
