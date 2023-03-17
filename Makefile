include .env
export 

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
