# Custom ArgoCD Image

This repo contains a Dockerfile for a [custom ArgoCD Repo image](https://argoproj.github.io/argo-cd/operator-manual/custom_tools/), which includes custom plugins for rendering Kubernetes yamls, detailed below.

This image is used by the [Analysis Platforms ArgoCD instance](https://ap-argocd.dsp-devops.broadinstitute.org).

## Plugins

#### `legacy-configs`

Renders Consul templates from [firecloud-develop](https://github.com/broadinstitute/firecloud-develop) as Kubernetes secrets.

Example usage:

    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata: <...>
    spec:
      <...>
      plugin:
        name: legacy-configs
        env:
        - name: APP_NAME
          value: cromwell
        - name: ENV
          value: dev
        - name: INSTANCE_TYPES
          value: cromwell1-frontend,cromwell1-runner,cromwell1-summarizer
        - name: RUN_CONTEXT
          value: live
        # Any other environment variables specified here will also propagate down to configure.rb


#### `helmfile`

Renderes Kubernetes resources for a repo containing a [helmfile](https://github.com/roboll/helmfile).

      plugin:
        name: helmfile
        env:
        - name: HELMFILE_ENV       # passed to helmfile with `--environment`
          value: dev
        - name: HELMFILE_SELECTOR  # passed to helmfile with `--selector`
          value: name=cromwell

#### `helm-values`

Makes it possible to configure an ArgoCD Application to monitor a Git repo containing values files and install a Helm chart from a Helm repo at the same time (for whatever reason ArgoCD's Helm support does not support this out of the box).

    plugin:
      name: helmfile
      env:
      # If you don't set these values they default to our in-house terra-helm repo
      - name: HELM_REPO_NAME
        value: argo
      - name: HELM_REPO_URL
        value: https://argoproj.github.io/argo-helm
      # Chart settings
      - name: HELM_CHART_NAME
        value: argocd
      - name: HELM_CHART_VERSION
        value: 1.2.3
      - name: HELM_CHART_NAMESPACE
        value: my-argocd-namespace
      - name: HELM_CHART_RELEASE
        value: my-argocd-release

## Building and publishing a new image

Docker images are automatically built by a Cloud Build trigger in the [dsp-artifact-registry](https://console.cloud.google.com/cloud-build/triggers?project=dsp-artifact-registry) project.

