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