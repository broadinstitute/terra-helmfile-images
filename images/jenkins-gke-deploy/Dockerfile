FROM alpine:3.11.5

# Specified in .github/workflows/build.yaml
ARG argocd_version
ARG vault_version
ARG os=linux
ARG arch=amd64

RUN apk add curl jq

# vault
RUN wget -O- "https://releases.hashicorp.com/vault/${vault_version}/vault_${vault_version}_${os}_${arch}.zip" | unzip - && \
    chmod +x vault && \
    mv vault /bin && \
    vault -version

# ArgoCD cli
RUN wget -O /bin/argocd "https://github.com/argoproj/argo-cd/releases/download/v${argocd_version}/argocd-${os}-${arch}" && \
  chmod +x /bin/argocd && \
  argocd help

COPY deploy.sh /deploy.sh

ENTRYPOINT ["/deploy.sh"]
