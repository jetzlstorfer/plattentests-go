include .env
export 

# Latest commit hash
GIT_SHA=$(shell git rev-parse HEAD)


token:
	go run cmd/token/main.go

web:
	cd webui && go run main.go

docker-web-build:
	docker build --build-arg GIT_SHA=$(GIT_SHA) -f ./webui/Dockerfile -t jetzlstorfer/plattentests-webui:latest .

docker-web-run:
	docker run -p 8081:8081 jetzlstorfer/plattentests-webui:latest
