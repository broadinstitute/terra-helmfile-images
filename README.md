# terra-helmfile-images

This repo builds Docker images related to the [terra-helmfile repo](https://github.com/broadinstitute/terra-helmfile) and Kubernetes deploy tools. These images depend on tools like `helmfile` and `ArgoCD` in order to work, and it's useful to be able to configure the versions of these tools in a common location. (Currently, the shared `cloudbuild.yml`).

* **`env-diff-action`**: [Used by the env-diff action](https://github.com/broadinstitute/terra-helmfile/blob/master/.github/actions/env-diff/action.yml) in the terra-helmfile repo to render manifests and comment on PRs with a diff of environment changes
* **`argocd-custom-image`**: [Used by ArgoCD](https://github.com/broadinstitute/terra-helm-definitions/search?q=argocd-custom) to render Kubernetes manifests during ArgoCD deploys
* **`jenkins-gke-deploy`**: [Used by Jenkins](https://github.com/broadinstitute/dsp-jenkins/search?q=jenkins-terra-gke-deploy) to trigger ArgoCD syncs during the Terra monolith release process

To update an image, open a PR and make any necessary changes. Once the PR is merged, update the tags of the images in the linked repositories.
