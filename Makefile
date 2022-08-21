#include .env

run:
	export `cat .env | xargs`
	go run main.go

build:
	go build cmd/crawler.go

run-local:
	make build
	func start

deploy:
	make build
	func azure functionapp publish plattentests-go


### OLD 

token:
	export `cat .env | xargs`
	echo $$SPOTIFY_ID
	echo ${SPOTIFY_ID}
	echo $$AWS_ACCESS_KEY_ID
	echo $$AWS_SECRET_ACCESS_KEY
	go run get-token/main.go

update:
	export `cat .env | xargs`
	env GOOS=linux go build .
	zip plattentests-go.zip ./plattentests-go
	aws lambda update-function-code \
		--region eu-north-1 \
		--function-name plattentests-go \
		--zip-file fileb://plattentests-go.zip
	rm -f plattentests-go plattentests-go.zip

update-env: 
	export `cat .env | xargs`
	aws lambda update-function-configuration \
		--region eu-north-1 \
		--function-name plattentests-go \
		--environment Variables="{SPOTIFY_ID=${SPOTIFY_ID},SPOTIFY_SECRET=${SPOTIFY_SECRET},PLAYLIST_ID=${PLAYLIST_ID},BUCKET=${BUCKET},TOKEN_FILE=${TOKEN_FILE},REGION=${REGION}}"

upload:
	export `cat .env | xargs`
	env GOOS=linux go build .
	zip plattentests-go.zip ./plattentests-go
	aws lambda create-function \
		--region eu-north-1 \
		--function-name plattentests-go \
		--memory 128 \
		--timeout 60 \
		--role arn:aws:iam::753078859875:role/lambda_execution \
		--runtime go1.x \
		--zip-file fileb://plattentests-go.zip \
		--handler plattentests-go \
		--environment Variables="{SPOTIFY_ID=${SPOTIFY_ID},SPOTIFY_SECRET=${SPOTIFY_SECRET},PLAYLIST_ID=${PLAYLIST_ID},BUCKET=${BUCKET},TOKEN_FILE=${TOKEN_FILE},REGION=${REGION}}"
	rm -f plattentests-go plattentests-go.zip
