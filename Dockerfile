FROM golang:1.25-alpine3.22
LABEL app=monteverdi
ENTRYPOINT ["/usr/bin/monteverdi"]
COPY config.json /usr/bin/
COPY monteverdi /usr/bin/