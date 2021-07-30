# terra-helmfile-images

[![codecov](https://codecov.io/gh/broadinstitute/terra-helmfile-images/branch/main/graph/badge.svg?token=QYQHL6UE6Y)](https://codecov.io/gh/broadinstitute/terra-helmfile-images)
[![Go Report Card](https://goreportcard.com/badge/github.com/broadinstitute/terra-helmfile-images/tools](https://goreportcard.com/report/github.com/broadinstitute/terra-helmfile-images/tools)

This repository contains accessory tooling for the [terra-helmfile repo](https://github.com/broadinstitute/terra-helmfile). It includes:
* Golang source code for various utilities for rendering manifests and publishing charts (see `tools/` subdirectory)
* Dockerfiles for images that interact with terra-helmfile (see `images/` subdirectory).

### Tools

* **`render`**: A convenience wrapper around Helmfile, used for rendering manifests

### Docker Images

* **`tools`**: [Used by terra-helmfile's render helper script](https://github.com/broadinstitute/terra-helmfile/blob/master/bin/render)
* **`env-diff-action`**: [Used by terra-helmfile's env-diff GitHub Action](https://github.com/broadinstitute/terra-helmfile/blob/master/.github/actions/env-diff/action.yml) to comment on PRs with a diff of environment changes
* **`jenkins-gke-deploy`**: [Used by Jenkins](https://github.com/broadinstitute/dsp-jenkins/search?q=jenkins-terra-gke-deploy) to trigger ArgoCD syncs during the Terra monolith release process
* **`jenkins-helmfile-version-query`**: [Used by Jenkins](https://fc-jenkins.dsp-techops.broadinstitute.org/job/gke-service-update/) to verify that a version update has successfully merged.
* **`render-action`**: [Used by terra-helmfile's render GitHub Action](https://github.com/broadinstitute/terra-helmfile/tree/master/.github/actions/render-action) to render manifests in different workflows.
* **`argocd-custom-image`**: [Used by ArgoCD](https://github.com/broadinstitute/terra-helm-definitions/search?q=argocd-custom) to render Kubernetes manifests during ArgoCD deploys

To update an image, open a PR and make any necessary changes. Everything except ArgoCD is pinned to the `main` tag, so the changes will be picked up automatically.
