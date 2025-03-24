# Этап сборки
FROM golang:1.24-alpine as builder
WORKDIR /build
# Копируем модули для загрузки зависимостей
COPY go.mod .
COPY go.sum .
RUN go mod download
# Копируем все файлы проекта
COPY *.go ./
# Собираем приложение, включая все необходимые .go файлы
RUN go build -o /main .
# Финальный этап
FROM alpine:3
COPY --from=builder /main /bin/main
ENTRYPOINT ["/bin/main"]