# API Gateway

基于 Go (Gin) 原生搭建的轻量级、高性能分布式微服务流量网关。

## 📖 项目简介
本项目作为整个微服务集群的“最前端门户”（Backend For Frontend / Edge Gateway），主要负责南北向流量（公网到内网）的调度与安全控制。
通过将鉴权、限流、链路追踪等 CPU 密集型与防线型逻辑收拢于网关层，真正实现了内网各个下游微服务节点（User-Platform, Go-Note, Game-Server 等）的“边界隔离与降负荷无状态化”解耦。

## ✨ 核心特性
- **高性能代理转发**：基于官方标准库 `httputil.ReverseProxy`，附加手动深调参的 HTTP/2 级连接池 (`http.Transport`)，实现零拷贝、万级并发的极速吞吐，性能比肩商用网关。
- **全局 JWT 鉴权网络**：作为集群唯一的边防检查站，拦截非法请求，并将解码后的有效上下文转化为 `X-User-Id` Header 后，向底层的微服务集群透明透传。
- **免鉴权白名单机制**：通过中间件层面的统一字典过滤（例如 `/login`、`/register` 等接口），规避了底层 Trie 树路由重名冲突。
- **全链路追踪 (Trace)**：在请求入口处自动生成生命周期级 `UUID` (TraceID)，携带进全链路日志中，帮助极速定位排障。
- **云原生配置覆盖**：底层集成 Viper，完美支持外部容器环境变量（Env）一键覆写路由指向，0 侵入代码即可无缝切入 K8s/Docker 组网拓扑。

## 🗂 目录结构
```text
api-gateway/
├── cmd/
│   └── server/          # 应用程序启动入口及全量路由装配中心 (main.go)
├── configs/             # YAML 本地配置文件目录
├── internal/
│   ├── auth/            # JWT 解析与 Token 验证引擎工具包
│   ├── config/          # 基于 Viper 映射的全局应用结构体
│   ├── middleware/      # 网关四度防线 (JWTAuth, GinLogger, Trace, Recovery)
│   └── proxy/           # 深度改装连接数与保活池的反向代理分发引擎
├── docker-compose.yaml  # 容器级联与网络编排定义
├── Dockerfile           # 面向云原生的多阶段容器构建脚本
└── go.mod               # 模块依赖表 (共享 common 基础设施依赖)
```

## 🚀 路由拓扑矩阵 (Routing Matrix)
当前网关支持对前端流量基于白名单 + 二级路由的精准分发，所有匹配服务均自动受 JWT 网关护卫：

### 用户生态层 (→ target: `user-platform`)
* `[免鉴权]` `POST /api/v1/users/login` -> 越过大门直达 Controller
* `[免鉴权]` `POST /api/v1/users/register` -> 同上
* `[受护卫]` `ANY  /api/v1/users/*` -> 网关验签 -> 下发 `X-User-Id` 头 -> 转发

### 笔记系统层 (→ target: `go-note`)
* `[受护卫]` `ANY  /api/v1/me/pastes` -> 验证身份流转至笔记业务集群
* `[受护卫]` `ANY  /api/v1/pastes/*` -> 同上

## 🛠 启动与极速部署指南

### 方式一：本地 Go 原生极速测试
依托 `go.work` 关联工作区，在网关目录下可直接热重载：
```bash
go run cmd/server/main.go
```
*网关实例将挂载于本机的 `localhost:8000` 端口监听请求。*

### 方式二：Docker 容器全景网络联排
网关的 `docker-compose.yaml` 已自带动态路由网络配置，它会在隔离网桥中将流量甩向名为 `user-http` / `go-note-http` 的兄弟容器。
```bash
docker-compose up -d --build
```

## 🛡️ 架构师视角：Zero-Trust 内网零信任重树
**“网关拔剑，微服务收刀”**
所有隐身在网关背后的底层微服务模块，**不再需要（也严禁）自己浪费 CPU 计算资源去解密和校对 JWT**。
微服务的唯一工作就是：无脑信任并从 Request Header 中汲取 `X-User-Id` 进而推动商业逻辑推进。它保证了即便未来基建大升级（如直接废弃该网关切入 APISIX 甚至是 Service Mesh Envoy），你的所有后端项目代码都可做到 **0 行修改，免密热迁移**。
