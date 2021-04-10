FROM golang:alpine AS build
WORKDIR $GOPATH/src/github.com/lukechampine/muse
COPY . .
ENV CGO_ENABLED=0
RUN apk -U --no-cache add bash upx git gcc make \
    && make static \
    && upx /go/bin/muse

FROM scratch
COPY --from=build /go/bin/muse /muse
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
CMD ["/muse"]
