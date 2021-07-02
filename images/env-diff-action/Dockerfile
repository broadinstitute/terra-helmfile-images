# Container image that runs your code
FROM python:3-alpine

ARG os=linux
ARG arch=amd64

# Add required OS packages
# jq, rsync needed for action.sh
# bash needed for `render` script
RUN apk --update add --no-cache bash jq rsync

# Create /tools directory
RUN mkdir -p /tools/bin

# Add /tools/bin/ to PATH
ENV PATH="/tools/bin:${PATH}"

# Python dependencies
COPY requirements.txt /tools/requirements.txt
RUN pip3 install -r /tools/requirements.txt

COPY env-differ   /tools/bin/env-differ
COPY pull-request /tools/bin/pull-request
COPY action.sh /action.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/action.sh"]
