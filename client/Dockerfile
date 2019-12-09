# build stage
FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ADD *.go /src/
RUN cd /src && \
    go get -u golang.org/x/net/proxy && \
    go get -u github.com/gorilla/websocket && \
    go build -o client && chmod +x client

# final stage
FROM alpine:edge
EXPOSE 8080
WORKDIR /app
COPY --from=build-env /src/client /app/
ADD https://github.com/just-containers/s6-overlay/releases/download/v1.21.8.0/s6-overlay-amd64.tar.gz /tmp/
RUN tar xzf /tmp/s6-overlay-amd64.tar.gz -C / && \
    mkdir -p /etc/services.d/client && \
    mkdir -p /etc/services.d/tor && \
    mkdir -p /var/run/tor && \
    mkdir -p /etc/fix-attrs.d/ && \
    apk add --no-cache tor
COPY tor.fix /etc/fix-attrs.d/01-tor
COPY client.run /etc/services.d/client/run
COPY tor.run /etc/services.d/tor/run
ENTRYPOINT ["/init"]