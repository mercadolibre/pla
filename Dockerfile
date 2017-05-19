FROM golang

RUN apt-get update && apt-get install -y bzip2 && \
    wget http://upx.sourceforge.net/download/upx-3.91-amd64_linux.tar.bz2 && \
    tar -xjf upx-3.91-amd64_linux.tar.bz2 && \
    mv upx-3.91-amd64_linux/upx /bin/upx && \
    go get -ldflags='-s -w' github.com/mercadolibre/pla && \
    upx --force bin/pla


FROM progrium/busybox

COPY --from=0 /go/bin/pla /bin/pla

ENTRYPOINT ["pla"]
