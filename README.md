# terra-helmfile-images

[![codecov](https://codecov.io/gh/broadinstitute/terra-helmfile-images/branch/main/graph/badge.svg?token=QYQHL6UE6Y)](https://codecov.io/gh/broadinstitute/terra-helmfile-images)
[![Go Report Card](https://goreportcard.com/badge/github.com/broadinstitute/terra-helmfile-images/tools)](https://goreportcard.com/report/github.com/broadinstitute/terra-helmfile-images/tools)

This repository contains Dockerfiles for images that interact with the [terra-helmfile repo](https://github.com/broadinstitute/terra-helmfile).

### Docker Images

* **`jenkins-gke-deploy`**: [Used by Jenkins](https://github.com/broadinstitute/dsp-jenkins/search?q=jenkins-terra-gke-deploy) to trigger ArgoCD syncs during the Terra monolith release process
* **`jenkins-helmfile-version-query`**: [Used by Jenkins](https://fc-jenkins.dsp-techops.broadinstitute.org/job/gke-service-update/) to verify that a version update has successfully merged.
* **`argocd-custom-image`**: [Used by ArgoCD](https://github.com/broadinstitute/terra-helm-definitions/search?q=argocd-custom) to render Kubernetes manifests during ArgoCD deploys

To update an image, open a PR and make any necessary changes. Everything except ArgoCD is pinned to the `main` tag, so the changes will be picked up automatically.

### Thelma

Looking for Thelma? It's been moved to a [separate repo](https://github.com/broadinstitute/thelma)
