# syntax=docker/dockerfile:1
# ======== 构建阶段 ========
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 设置国内 Go 代理
ENV GOPROXY=https://goproxy.cn,direct

# 先拷贝依赖清单，提高缓存命中率
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 再拷贝当前服务源码
COPY . .

# 编译服务（CGO_ENABLED=0 生成静态二进制，alpine 能直接运行）
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o api_gateway ./cmd/server

# ======== 运行阶段（镜像从 ~1GB 缩小到 ~30MB）========
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /app/api_gateway .
COPY --from=builder /app/configs ./configs

EXPOSE 8000
CMD ["./api_gateway"]
