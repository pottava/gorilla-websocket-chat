FROM golang:1.11.4-alpine3.8 AS build-env
RUN apk --no-cache add gcc musl-dev git
RUN go get -u github.com/golang/dep/...
WORKDIR /go/src/github.com/golang/dep
RUN git checkout v0.5.0 > /dev/null 2>&1
RUN go install github.com/golang/dep/...
WORKDIR /go/src/github.com/pottava
RUN git clone https://github.com/pottava/gorilla-websocket-chat.git
WORKDIR /go/src/github.com/pottava/gorilla-websocket-chat
RUN git checkout v1.0.1 > /dev/null 2>&1
WORKDIR /go/src/github.com/pottava/gorilla-websocket-chat/server
RUN dep ensure
RUN go build -a -installsuffix cgo -ldflags "-s -w"
RUN mv server /app

FROM alpine:3.8
COPY --from=build-env /app /app
ENTRYPOINT ["/app"]
