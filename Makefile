# Run go fmt against code
fmt:
	go fmt ./...
	gofmt -s -w .

# Run go vet against code
vet:
	go vet ./...

# Run go mod tidy
tidy:
	go mod tidy

# Run tests
test: tidy fmt vet
	go test ./...  -coverprofile=coverage.out
	go tool cover -func=coverage.out

image:
	podman build -t rest2mqtt --build-arg ARG_GOARCH=arm --build-arg ARG_GOARM=7 .
	podman save rest2mqtt > rest2mqtt.tar

.PHONY: post
post:
	curl -s -o /dev/null -w "%{http_code}\n" --data "{\"token\":\"${REST2MQTT_TOKEN}\", \"topic\":\"api/test\", \"payload\": \"test\",\"qos\":0, \"retained\":false}" localhost:8080/v1/mqtt
post-bearer:
	curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer ${REST2MQTT_TOKEN}" --data "{\"topic\":\"api/test\", \"payload\": \"test\",\"qos\":0, \"retained\":false}" localhost:8080/v1/mqtt
