include .env
export 

# Latest commit hash
GIT_SHA=$(shell git rev-parse HEAD)


token:
	go run cmd/token/main.go

run:
	cd webui && go run main.go

lint:
	golangci-lint run ./...

test:
	go test ./...

docker-web-build:
	docker build --build-arg GIT_SHA=$(GIT_SHA) -f ./webui/Dockerfile -t jetzlstorfer/plattentests-webui:latest .

docker-web-run:
	docker run -p 8081:8081 jetzlstorfer/plattentests-webui:latest
