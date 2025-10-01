FROM golang:1.25-alpine3.22
LABEL app=monteverdi
ENTRYPOINT ["/monteverdi"]
COPY config.json /
COPY monteverdi /