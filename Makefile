include .env
export 

# Latest commit hash
GIT_SHA=$(shell git rev-parse HEAD)

run:
	go run main.go

install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.51.2

lint:
	golangci-lint run

run-function:
	env GOOS=linux go build .
	func start

upload-function:
	env GOOS=linux go build .
	func azure functionapp publish plattentests-go

token:
	go run cmd/token/main.go

web:
	cd webui && go run main.go

docker-web-build:
	docker build --build-arg GIT_SHA=$(GIT_SHA) -f ./webui/Dockerfile -t jetzlstorfer/plattentests-webui:latest .

docker-web-run:
	docker run -p 8080:8080 jetzlstorfer/plattentests-webui:latest
