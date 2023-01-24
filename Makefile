include .env
export 

run:
	go run main.go

run-function:
	env GOOS=linux go build .
	func start

upload-function:
	env GOOS=linux go build .
	func azure functionapp publish plattentests-go

token:
	go run get-token/main.go
