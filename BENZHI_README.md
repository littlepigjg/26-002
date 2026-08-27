# UBAAS (User Behavior Analysis as a Service)

基于纯 Go 标准库实现的用户行为路径分析系统（埋点数据后端）。

## 项目概述

UBAAS 是一个轻量级的用户行为分析平台，提供事件追踪、会话管理、路径分析、转化漏斗、维度筛选和数据导出等核心功能。

### 技术特性

- **纯 Go 标准库**: 零第三方依赖，使用 Go 1.26 标准库实现全部功能
- **RESTful API**: 提供完整的 HTTP 接口，支持事件采集、查询、统计、导出
- **内存存储**: 高效的内存数据存储，支持 TTL 缓存和 LRU 淘汰
- **优雅关闭**: 支持 SIGINT/SIGTERM 信号处理，确保数据完整性
- **结构化日志**: 支持 JSON 和文本两种日志格式，支持文件和控制台输出
- **参数校验**: 全面的请求参数验证，确保数据质量
- **安全中间件**: CORS、速率限制、请求超时、Panic 恢复
- **Docker 支持**: 多阶段 Docker 构建，最小化镜像体积

## 项目结构

```
ubaas/
├── cmd/
│   └── server/
│       └── main.go          # 应用入口
├── internal/
│   ├── config/
│   │   └── config.go         # 配置管理
│   ├── handler/
│   │   ├── event_handler.go      # 事件 API
│   │   ├── session_handler.go    # 会话 API
│   │   ├── path_handler.go       # 路径 API
│   │   ├── stats_handler.go      # 统计 API
│   │   ├── conversion_handler.go # 转化 API
│   │   ├── dimension_handler.go  # 维度 API
│   │   ├── export_handler.go     # 导出 API
│   │   ├── health_handler.go     # 健康检查
│   │   ├── router.go             # 路由注册
│   │   └── helpers.go            # handler 辅助函数
│   ├── middleware/
│   │   ├── middleware.go    # 日志/恢复/CORS/超时中间件
│   │   └── security.go      # 安全头/内容类型校验
│   ├── model/
│   │   ├── event.go         # 事件模型
│   │   ├── session.go       # 会话模型
│   │   ├── path.go          # 路径模型
│   │   ├── conversion.go    # 转化模型
│   │   ├── dimension.go     # 维度模型
│   │   ├── user.go          # 用户维度模型
│   │   ├── filter.go        # 过滤常量
│   │   ├── export.go        # 导出模型
│   │   ├── metrics.go       # 指标模型
│   │   └── common.go        # 通用错误/ID生成器
│   ├── service/
│   │   ├── event_service.go       # 事件服务
│   │   ├── session_service.go     # 会话服务
│   │   ├── path_service.go        # 路径服务
│   │   ├── conversion_service.go  # 转化服务
│   │   ├── stats_service.go       # 统计服务
│   │   ├── dimension_service.go   # 维度服务
│   │   ├── export_service.go     # 导出服务
│   │   ├── aggregation_service.go # 聚合服务
│   │   ├── query_service.go      # 查询服务
│   │   └── scheduler.go           # 定时任务调度
│   └── store/
│       ├── memory_store.go            # 内存存储核心
│       ├── session_store.go           # 会话存储
│       ├── path_store.go              # 路径存储
│       ├── conversion_store.go        # 转化存储
│       ├── conversion_store_ext.go    # 转化存储扩展
│       ├── stats_store.go             # 统计存储
│       └── user_store.go              # 用户存储
├── pkg/
│   ├── logger/
│   │   ├── logger.go        # 结构化日志
│   │   └── format.go        # JSON/文件日志格式化
│   ├── response/
│   │   ├── response.go      # 统一响应格式
│   │   └── handler.go       # 响应辅助函数
│   ├── validator/
│   │   ├── validator.go     # 请求参数校验
│   │   └── sanitize.go      # 输入清洗
│   ├── timeutil/
│   │   └── time.go          # 时间工具函数
│   ├── cache/
│   │   ├── cache.go         # TTL 缓存
│   │   └── lru.go           # LRU 缓存实现
│   └── pool/
│       └── pool.go          # sync.Pool 封装
├── web/
│   ├── index.html           # 前端展示页面
│   ├── css/style.css        # 样式
│   └── js/app.js            # 前端逻辑
├── go.mod                   # Go 模块定义
├── benzhi.Dockerfile        # Docker 构建文件
├── build_benzhi_docker.sh   # Docker 构建脚本
├── BENZHI_README.md         # 本文件
├── BUG_CATALOG.md           # 缺陷候选清单
└── .dockerignore            # Docker 忽略文件
```

## 快速开始

### 编译运行

```bash
# 编译
go build -o ubaas-server ./cmd/server/

# 运行
./ubaas-server
```

服务器默认监听 `0.0.0.0:8080`。

### 使用环境变量配置

```bash
export SERVER_PORT=9090
export SERVER_HOST=0.0.0.0
export LOGGING_LEVEL=INFO
./ubaas-server
```

### Docker 构建

```bash
# 使用构建脚本
./build_benzhi_docker.sh

# 或手动构建
docker build -f benzhi.Dockerfile -t ubaas-server:latest .

# 运行容器
docker run -d -p 8080:8080 ubaas-server:latest
```

## API 接口

### 健康检查
```
GET /health
```
返回服务状态和系统指标。

### 事件 API
```
POST /api/events                    # 创建单个事件
POST /api/events/batch               # 批量创建事件
GET  /api/events                    # 查询事件列表
GET  /api/events/{id}               # 获取单个事件
DELETE /api/events/{id}             # 删除事件
```

### 会话 API
```
GET  /api/sessions                  # 查询会话列表
GET  /api/sessions/{id}             # 获取会话详情
GET  /api/sessions/stats            # 会话统计
```

### 路径 API
```
GET  /api/paths/popular             # 热门路径
GET  /api/paths/pages/popular       # 热门页面
GET  /api/paths/{id}                # 获取路径详情
```

### 统计 API
```
GET  /api/stats/overview            # 数据概览
GET  /api/stats/trends              # 趋势数据
GET  /api/stats/aggregation         # 聚合数据
```

### 转化 API
```
POST /api/conversions/goals         # 创建转化目标
GET  /api/conversions/goals         # 获取转化目标列表
GET  /api/conversions/trends        # 转化趋势
GET  /api/conversions/funnel        # 转化漏斗分析
```

### 维度 API
```
POST /api/dimensions/filter         # 维度筛选
GET  /api/dimensions/breakdown     # 维度分解
```

### 导出 API
```
POST /api/exports                   # 创建导出任务
GET  /api/exports/{id}              # 查询导出状态
```

## 配置说明

所有配置通过环境变量传入：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| SERVER_HOST | 服务监听地址 | 0.0.0.0 |
| SERVER_PORT | 服务端口 | 8080 |
| SERVER_READ_TIMEOUT | 读超时（秒） | 30 |
| SERVER_WRITE_TIMEOUT | 写超时（秒） | 30 |
| SERVER_IDLE_TIMEOUT | 空闲超时（秒） | 120 |
| SERVER_SHUTDOWN_TIMEOUT | 关闭超时（秒） | 30 |
| LOGGING_LEVEL | 日志级别 | INFO |
| LOGGING_FORMAT | 日志格式 | text |
| LOGGING_OUTPUT | 日志输出 | stdout |

## 依赖

- Go 1.26+
- 纯标准库（零第三方依赖）

## 文档

- [BUG_CATALOG.md](BUG_CATALOG.md) - 缺陷候选清单（30 个）

## 许可证

MIT License
