FROM golang:1.17-alpine as BuildStage

WORKDIR /app

COPY . .
RUN go mod download

RUN cd webui && go build -o /plattentests-web

FROM alpine:3.17

WORKDIR /

COPY --from=BuildStage /app/webui/templates ./templates
COPY --from=BuildStage /app/webui/assets ./assets
COPY --from=BuildStage /plattentests-web .

EXPOSE 8081

CMD [ "/plattentests-web" ]