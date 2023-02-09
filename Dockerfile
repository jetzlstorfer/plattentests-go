FROM golang:1.17-alpine as BuildStage

WORKDIR /app

COPY . .
RUN go mod download

RUN go build -o /plattentests-go

FROM alpine:3.17

WORKDIR /

COPY --from=BuildStage /plattentests-go .

EXPOSE 8080

CMD [ "/plattentests-go" ]
