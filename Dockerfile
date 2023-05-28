FROM golang:1.20.4-alpine3.17 AS build
WORKDIR /build
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /app

FROM alpine:latest
WORKDIR /
COPY --from=build /app /app
ENTRYPOINT ["/app"]