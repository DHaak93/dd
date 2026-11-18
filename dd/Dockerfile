# Base image. Supervisor 2026.04+ no longer passes BUILD_FROM (build.yaml is
# deprecated), so give it a default: the multi-arch Home Assistant base image,
# same Alpine release as before.
ARG BUILD_FROM=ghcr.io/home-assistant/base:3.22

# Build the Go binaries from this fork's source. Home Assistant builds the
# add-on locally with only the dd/ folder as build context, so the source is
# cloned here (see README "Building" for a prebuilt-image alternative).
FROM golang:1.23-alpine AS build
RUN apk add --no-cache git
ARG DD_REPO=https://github.com/DHaak93/dd.git
ARG DD_REF=master
# Passed by the Supervisor: changing the add-on version invalidates the cached
# clone below, so every change must bump the version in config.yaml.
ARG BUILD_VERSION
RUN echo "Building dd ${BUILD_VERSION} from ${DD_REPO}@${DD_REF}"
RUN git clone --depth 1 --branch "${DD_REF}" "${DD_REPO}" /src
WORKDIR /src
ENV CGO_ENABLED=0
RUN go build -trimpath -o /out/register ./bin/register \
 && go build -trimpath -o /out/action ./bin/action \
 && go build -trimpath -o /out/haus ./bin/haus

FROM ${BUILD_FROM}

LABEL \
  org.opencontainers.image.title="Home Assistant Add-on: SmartDoor MQTT Bridge" \
  org.opencontainers.image.description="SmartDoor garage door integration with MQTT and position control support" \
  org.opencontainers.image.source="https://github.com/DHaak93/dd" \
  org.opencontainers.image.licenses="Apache-2.0"

# Copy root filesystem and the binaries
COPY rootfs /
COPY --from=build /out/ /usr/bin/dd/

# Ensure scripts and binaries are executable
RUN chmod +x /etc/cont-init.d/* /etc/services.d/dd/run /etc/services.d/dd/finish \
 && chmod +x /usr/bin/dd/action /usr/bin/dd/haus /usr/bin/dd/register

# Set s6-overlay environment variables
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_CMD_WAIT_FOR_SERVICES=1

# Set the s6-overlay entrypoint
ENTRYPOINT ["/init"]
CMD []
