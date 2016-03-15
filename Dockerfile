FROM golang:alpine

RUN mkdir /app

WORKDIR /app

ENV GO15VENDOREXPERIMENT=1

ADD . /go/src/github.com/sschepens/pla

RUN apk add --update git && \
  cd /go/src/github.com/sschepens/pla && \
  go get && \
  go build && \
  mv /go/src/github.com/sschepens/pla /app/pla && \
  rm -fr /go/* && \
  rm -fr /usr/local/go && \
  apk del --purge git && rm -rf /var/cache/apk/*

ENTRYPOINT ["./pla/pla"]

