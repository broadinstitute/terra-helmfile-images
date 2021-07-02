# Container image that runs your code
FROM alpine:3

# These are specified in cloudbuild.yaml
ARG helmfile_version
ARG helm_version

# Add required OS packages
# needed for entrypoint.sh
# bash needed for `render` script
RUN apk --update add --no-cache bash

# Create /tools directory
RUN mkdir -p /tools/bin

# Add /tools/bin/ to PATH
ENV PATH="/tools/bin:${PATH}"

# Copy compiled Go tools into /tools/bin directory
COPY --from=tools-local /tools/bin /tools/bin

COPY entrypoint.sh /entrypoint.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/entrypoint.sh"]
