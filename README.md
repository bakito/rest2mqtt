# rest to MQTT bridge

## Post Request

```json
{
  "token": "<token>",
  "topic": "api/test",
  "payload": "test",
  "qos": 0,
  "retained": false
}
```

```bash
curl --data '{"token":"<token>", "topic":"api/test", "payload": "test","qos":0, "retained":false }' localhost:8080/v1/mqtt

curl -H "Authorization: Bearer <token>" --data '{"token":"token", "topic":"api/test", "payload": "test","qos":0, "retained":false }' localhost:8080/v1/mqtt
```

## Build for rpi

```bash
podman build -t rest2mqtt --build-arg ARG_GOARCH=arm --build-arg ARG_GOARM=7 .
```