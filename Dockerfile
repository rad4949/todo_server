FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o todo_server ./cmd/main.go
RUN go build -o notification ./cmd/notification/main.go

FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/todo_server ./todo_server
COPY --from=builder /app/.env ./.env
COPY --from=builder /app/notification ./notification

EXPOSE 8080

CMD ["./todo_server"]