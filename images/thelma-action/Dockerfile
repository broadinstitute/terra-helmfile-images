# Specified in .github/workflows/build.yaml
ARG tools_image

# Add tools image as empty build stage so we can refer
# to it laster in a COPY command
FROM ${tools_image} AS tools

FROM alpine:3

# Add required OS packages
# git needed for thelma github actions
RUN apk --update add --no-cache git

# Create /tools directory
RUN mkdir -p /tools/bin

# Add /tools/bin/ to PATH
ENV PATH="/tools/bin:${PATH}"

# Copy compiled Go tools into /tools/bin directory
COPY --from=tools /tools/bin /tools/bin

ENTRYPOINT ["/tools/bin/thelma"]
