FROM golang:1.16-alpine as builder
ENV CGO_ENABLED=0
WORKDIR /go/src/
COPY . .
RUN go build -tags netgo -ldflags '-w' -v -o /usr/local/bin/function ./

FROM alpine:latest
COPY --from=builder /usr/local/bin/function /usr/local/bin/function
CMD ["function"]
