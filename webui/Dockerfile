FROM golang:1.25-alpine AS buildstage

WORKDIR /app

COPY . .
RUN go mod download

RUN cd webui && go build -o /plattentests-web

FROM alpine:3.19

WORKDIR /

COPY --from=buildstage /app/webui/templates ./templates
COPY --from=buildstage /app/webui/assets ./assets
COPY --from=buildstage /plattentests-web .

# see https://gist.github.com/bodhi/2c2152bb69dd2ef769002bf175a3d509

ARG GIT_SHA
ENV GIT_SHA=${GIT_SHA}

EXPOSE 8081

CMD [ "/plattentests-web" ]