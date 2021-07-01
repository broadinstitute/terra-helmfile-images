# terra-helmfile-images

This repo builds Docker images related to the [terra-helmfile repo](https://github.com/broadinstitute/terra-helmfile) and Kubernetes deploy tools. These images depend on tools like `helmfile` and `ArgoCD` in order to work, and it's useful to be able to configure the versions of these tools in a common location. (Currently, the shared `cloudbuild.yml`).

### Docker images
* **`env-diff-action`**: [Used by terra-helmfile's env-diff GitHub Action](https://github.com/broadinstitute/terra-helmfile/blob/master/.github/actions/env-diff/action.yml) to comment on PRs with a diff of environment changes
* **`argocd-custom-image`**: [Used by ArgoCD](https://github.com/broadinstitute/terra-helm-definitions/search?q=argocd-custom) to render Kubernetes manifests during ArgoCD deploys
* **`jenkins-gke-deploy`**: [Used by Jenkins](https://github.com/broadinstitute/dsp-jenkins/search?q=jenkins-terra-gke-deploy) to trigger ArgoCD syncs during the Terra monolith release process
* **`jenkins-helmfile-version-query`**: [Used by Jenkins](https://fc-jenkins.dsp-techops.broadinstitute.org/job/gke-service-update/) to verify that a version update has successfully merged.
* **`render`**: [Used by terra-helmfile's render GitHub Action](https://github.com/broadinstitute/terra-helmfile/tree/master/.github/actions/render-action) to render manifests in different workflows.

To update an image, open a PR and make any necessary changes. Once the PR is merged, update the tags of the images in the linked repositories.
