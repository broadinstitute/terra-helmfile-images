# Specified in .github/workflows/build.yaml
ARG tools_image

# Add tools image as empty build stage so we can refer
# to it laster in a COPY command
FROM ${tools_image} AS tools

# Container image that runs your code
FROM alpine:3

# Add required OS packages
# needed for entrypoint.sh
# bash needed for `render` script
RUN apk --update add --no-cache bash

# Create /tools directory
RUN mkdir -p /tools/bin

# Add /tools/bin/ to PATH
ENV PATH="/tools/bin:${PATH}"

# Copy compiled Go tools into /tools/bin directory
COPY --from=tools /tools/bin /tools/bin

COPY entrypoint.sh /entrypoint.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/entrypoint.sh"]
