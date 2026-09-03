# === ЭТАП 1: Сборка бинарного файла ===
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

# Рабочая директория внутри контейнера
WORKDIR /src

# Копируем go.mod и go.sum из текущей папки (а текущей теперь будет IdGenerator)
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь остальной код из папки IdGenerator в корень /src контейнера
COPY . .

# Собираем приложение.
# Так как папка cmd теперь лежит прямо в корне контекста,
# путь до main.go сокращается:
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/idgenerator \
    ./cmd/idgenservice/main.go

# === ЭТАП 2: Финальный минимальный образ ===
FROM alpine:3.20

RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Забираем готовый файл из первого этапа
COPY --from=builder /app/idgenerator .

EXPOSE 50051

CMD ["./idgenerator"]
# docker build -t andrew/idgen:latest .