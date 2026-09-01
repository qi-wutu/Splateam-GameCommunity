# ========== 构建阶段 ==========
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# ========== 运行阶段 ==========
FROM alpine:3.20

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/server .
COPY config/.env config/.env

EXPOSE 8080
CMD ["./server"]
