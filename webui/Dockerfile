FROM golang:1.23-alpine as BuildStage

WORKDIR /app

COPY . .
RUN go mod download

RUN cd webui && go build -o /plattentests-web

FROM alpine:3.17

WORKDIR /

COPY --from=BuildStage /app/webui/templates ./templates
COPY --from=BuildStage /app/webui/assets ./assets
COPY --from=BuildStage /plattentests-web .

# see https://gist.github.com/bodhi/2c2152bb69dd2ef769002bf175a3d509

ARG GIT_SHA
ENV GIT_SHA=${GIT_SHA}

EXPOSE 8081

CMD [ "/plattentests-web" ]