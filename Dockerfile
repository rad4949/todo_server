# Вибираємо базовий образ з Go
FROM golang:1.25.7 AS builder

# Встановлюємо робочий каталог у контейнері
WORKDIR /app

# Копіюємо файли go.mod і go.sum
COPY go.mod go.sum ./

# Завантажуємо всі залежності
RUN go mod tidy

# Копіюємо всі файли програми в контейнер
COPY . .

# Компільємо програму
RUN go build -o todo_server ./main.go

# Вказуємо порт, на якому програма буде слухати
EXPOSE 8080

# Команда для запуску програми в контейнері
CMD ["./todo_server"]