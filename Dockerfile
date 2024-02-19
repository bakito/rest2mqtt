FROM golang:1.22-bullseye as builder
# install xz and upx
RUN apt-get update && \
    apt-get install -y xz-utils upx && \
    rm -rf /var/lib/apt/lists/*

# setup the working directory
WORKDIR /go/src/app
ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux

COPY . /go/src/app/

# build the source
RUN go build -a -installsuffix cgo -o rest2mqtt .
RUN upx --ultra-brute -q rest2mqtt

FROM scratch

CMD ["/opt/go/rest2mqtt"]
HEALTHCHECK CMD ["/opt/go/rest2mqtt", "-healthz"]

EXPOSE 8080

COPY --from=builder /go/src/app/rest2mqtt /opt/go/
WORKDIR /opt/go/
USER 1001

