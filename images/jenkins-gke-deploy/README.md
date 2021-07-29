# jenkins-terra-gke-deploy

A Docker image for trigging Terra GKE deploys from the [legacy FireCloud Jenkins servers](https://github.com/broadinstitute/dsp-jenkins).

Currently, that means triggering an [ArgoCD](https://ap-argocd.dsp-devops.broadinstitute.org/) sync for the appropriate applications.

## Building and publishing a new image

Docker images are automatically built, tagged with `$BRANCH` and `$BRANCH-$GIT_SHA`, and pushed to the `dsp-artifact-registry` GCP project.

The [gke-deploy](https://fc-jenkins.dsp-techops.broadinstitute.org/job/gke-deploy/) job in Jenkins is configured to pull the latest `master` image.

## Running stable published image

    #
    # Set VAULT_TOKEN to a valid Vault token
    # Set ENV to the name of the target environment, eg "dev"
    # Set PROJECT to the name of an application, eg. "cromwell"
    #
    IMAGE="us-central1-docker.pkg.dev/dsp-artifact-registry/terra-helmfile/argocd-custom-image"
    docker pull "$IMAGE"
    docker run --rm -it \
      -e VAULT_TOKEN=$( cat ~/.vault-token ) \
      -e ENV="${env}" \
      "${IMAGE}" "${PROJECT}"

## Generating an ArgoCD token and storing it in Vault

Jenkins authenticates to ArgoCD using the local user `fc-jenkins` (`fcprod-jenkins` for prod), with an API token that is currently stored in Vault. It's possible to generate a new token as follows:

    # Login with your GitHub account (you must be an ArgoCD admin)
    argocd login ap-argocd.dsp-devops.broadinstitute.org --grpc-web --sso

    # Generate a new token for `fc-jenkins` user
    argocd account generate-token --account fc-jenkins

Then, save to `secret/devops/ci/argocd/jenkins-terra-sync-token` in Vault under the key `token` (`secret/suitable/argocd/jenkins-terra-sync-token` for fcprod-jenkins).
