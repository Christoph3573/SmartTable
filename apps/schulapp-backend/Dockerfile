FROM golang:1.26-alpine AS builder

RUN apk add --no-cache curl

RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.2/migrate.linux-amd64.tar.gz | tar xz && \
    mv migrate /usr/local/bin/migrate

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bin/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/migrations /app/migrations

WORKDIR /app

EXPOSE 8080

CMD ["/app/server"]
