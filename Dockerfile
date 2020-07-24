FROM golang:1.14 as builder
# install xz
RUN apt-get update && apt-get install -y \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*
# install UPX
RUN curl -L --progress-bar -o /usr/local/upx-3.96-amd64_linux.tar.xz https://github.com/upx/upx/releases/download/v3.96/upx-3.96-amd64_linux.tar.xz 2>&1 && \
    xz -d -c /usr/local/upx-3.96-amd64_linux.tar.xz | \
    tar -xOf - upx-3.96-amd64_linux/upx > /bin/upx && \
    chmod a+x /bin/upx
# install golint
RUN go get golang.org/x/lint/golint
# setup the working directory
WORKDIR /go/src/app
ARG ARG_GOARCH=amd64
ARG ARG_GOARM
ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=${ARG_GOARCH}\
  GOARM=${ARG_GOARM}

ADD . /go/src/app/

# golinth
RUN golint -set_exit_status  $(go list .)

# tests
RUN go test -coverprofile=coverage.out && \
    go tool cover -func=coverage.out && \
    rm -f coverage.out

# build the source
RUN go build -a -installsuffix cgo -o main .
RUN upx main

FROM scratch

CMD ["/opt/go/main"]

EXPOSE 8000

COPY --from=builder /go/src/app/main /opt/go/
WORKDIR /opt/go/
USER 1001

