FROM golang:1.20-alpine as BuildStage

WORKDIR /app

COPY . .
RUN go mod download

# see https://gist.github.com/bodhi/2c2152bb69dd2ef769002bf175a3d509

ARG GIT_SHA

RUN cd webui && go build -ldflags "-X git.sha=${GIT_SHA}" -o /plattentests-web

FROM alpine:3.17

WORKDIR /

COPY --from=BuildStage /app/webui/templates ./templates
COPY --from=BuildStage /app/webui/assets ./assets
COPY --from=BuildStage /plattentests-web .

EXPOSE 8081

CMD [ "/plattentests-web" ]