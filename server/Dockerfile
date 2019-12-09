# build stage
FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ADD . /src
RUN cd /src && go get -u github.com/gorilla/websocket && go build -o goapp

# final stage
FROM alpine
EXPOSE 8080
WORKDIR /app
COPY --from=build-env /src/goapp /app/
ADD permutations.txt /app/
ENTRYPOINT ./goapp