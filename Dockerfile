FROM golang:1.27-alpine AS compilar
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY plantillas ./plantillas
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /domo-board .

FROM alpine:3.24
# git para el repositorio de despliegue y el cliente de Docker para los stacks.
# El clon pertenece al usuario del servidor, no al de la imagen.
RUN apk add --no-cache git docker-cli docker-cli-compose \
 && git config --system --add safe.directory '*'
COPY --from=compilar /domo-board /usr/local/bin/domo-board
EXPOSE 8080
ENTRYPOINT ["domo-board"]
CMD ["servir"]
