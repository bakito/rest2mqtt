# rest to mqtt bridge

## Post Request

```bash
curl --data '{"token":"token", "topic":"api/test", "payload": "test","qos":0}' localhost:8080/v1/mqtt
```

## Build for rpi

```bash
podman build -t rest2mqtt --build-arg ARG_GOARCH=arm ARG_GOARM=7 .
```