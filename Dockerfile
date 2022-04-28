# Build

FROM golang:1.17-alpine AS build
ENV GOPATH /go
ENV GOBIN $GOPATH/bin

COPY . /go/src/github.com/doka-guide/search/
WORKDIR /go/src/github.com/doka-guide/search/
RUN go mod download
RUN go build -o /app/search

# Deploy

FROM alpine:latest
ARG APP_PORT=8080

VOLUME /app
WORKDIR /app
COPY --from=build /app ./
EXPOSE $APP_PORT
ENTRYPOINT ["/app/search"]