# ======== 构建阶段 ========
FROM golang:1.25-alpine AS builder

WORKDIR /build

# 设置国内 Go 代理
ENV GOPROXY=https://goproxy.cn,direct

# 先拷贝依赖文件，利用 Docker layer cache（go.mod 不变就不重新下载）
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码并编译（CGO_ENABLED=0 生成静态二进制，alpine 能直接运行）
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api_gateway ./cmd/server

# ======== 运行阶段（镜像从 ~1GB 缩小到 ~30MB）========
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /build/api_gateway .
COPY --from=builder /build/configs ./configs

EXPOSE 8000
CMD ["./api_gateway"]
