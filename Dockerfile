FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o context7-proxy .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/context7-proxy /app/context7-proxy
VOLUME /app/data
ENV DATABASE_PATH=/app/data/proxy.db
EXPOSE 8070
CMD ["/app/context7-proxy"]
