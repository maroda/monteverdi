FROM golang:1.25-alpine3.22
ARG TARGETPLATFORM
LABEL app=monteverdi
LABEL org.opencontainers.image.source=https://github.com/maroda/monteverdi
ENTRYPOINT ["/monteverdi"]
COPY config.json /
COPY $TARGETPLATFORM/monteverdi /