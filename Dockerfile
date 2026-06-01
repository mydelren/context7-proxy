FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o context7-proxy .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/context7-proxy .
VOLUME /app/data
ENV DATABASE_PATH=/app/data/proxy.db
EXPOSE 8070
CMD ["./context7-proxy"]
