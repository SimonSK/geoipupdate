FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git make bash

COPY . /geoipupdate

WORKDIR /geoipupdate

RUN make updater

FROM alpine:latest

WORKDIR /

COPY --from=builder /geoipupdate/bin/geoipupdate /usr/local/bin/

ADD ./conf/GeoIP.conf.default /usr/local/etc/GeoIP.conf

VOLUME ["/usr/local/share/GeoIP"]

ENTRYPOINT ["geoipupdate"]
