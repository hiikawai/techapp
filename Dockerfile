# ビルドステージ
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# 依存関係のダウンロード
RUN go mod download

# アプリケーションのビルド
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 実行ステージ
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/public ./public

# アプリケーションの実行
CMD ["./main"] 