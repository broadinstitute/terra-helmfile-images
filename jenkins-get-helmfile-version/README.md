# jenkins-get-helmfile-version

A Docker image for querying the current version of an application in terra-helmfile.

## Building and publishing a new image

Docker images are automatically built, tagged with `$BRANCH` and `$BRANCH-$GIT_SHA`, and pushed to the `dsp-artifact-registry` GCP project.

The [gke-service-update](https://fc-jenkins.dsp-techops.broadinstitute.org/job/gke-service-update/) job in Jenkins is configured to pull the latest `main` image.

## Running stable published image
    #
    # Set GITHUB_TOKEN to a valid GitHub personal access token w/ read access to terra-helmfile
    # Set APP to the name of an application, eg. "cromwell"
    #
    IMAGE="us-central1-docker.pkg.dev/dsp-artifact-registry/terra-helmfile/jenkins-get-helmfile-version"
    docker pull "$IMAGE"
    docker run --rm -it \
      -e GITHUB_TOKEN=$GITHUB_TOKEN \
      "${IMAGE}" "${APP}"

    # Report the version of leonardo in versions.yaml:
    docker run --rm -it \
      -e GITHUB_TOKEN=$GITHUB_TOKEN \
      "${IMAGE}" leonardo

### Overriding versions file:

It's possible to query another versions file like this:

    # Report alpha version of Leo
    docker run --rm -it \
      -e GITHUB_TOKEN=$GITHUB_TOKEN \
      -e VERSIONS_FILE=versions/alpha.yaml \
      "${IMAGE}" leonardo
