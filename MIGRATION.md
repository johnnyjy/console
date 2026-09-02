# Higress Console 后端迁移总结

本文档记录将 Higress Console 后端从 Java Spring Boot 迁移到 Go（GoFrame）的过程与结果。

## 1. 迁移目标

- 源项目：`/home/johnny/Desktop/higress-console`（Java Spring Boot）
- 目标项目：`/home/johnny/Desktop/console`（Go）
- 目标：替换 `java -jar /app/higress-console.jar` 后台服务进程，前端 API 端点与响应语义尽量保持一致。
- 框架：使用 GoFrame 统一路由与服务调用。
- K8s 客户端：使用 Go 版 `client-go`，不混合多语言。

## 2. 迁移范围

已迁移：

- 后端 HTTP 服务（全部业务 API 与静态资源服务）
- 会话/登录、系统初始化、用户、Dashboard、Grafana/AI Proxy 转发
- 路由、域名、服务来源、TLS 证书、消费者、代理服务器
- Wasm 插件及插件实例（global / domain / route / service 四个维度）
- AI 路由与 LLM Provider
- 前端静态资源服务（SPA fallback）

未迁移（按需求排除）：

- MCP Server
- Helm / Docker / 打包与集成脚本
- 前端代码本身（沿用 ice.js 既有构建产物，仅调整静态资源加载方式）

## 3. 技术栈对比

| 维度 | Java 版 | Go 版 |
|------|---------|-------|
| 语言/框架 | Java / Spring Boot | Go 1.24 / GoFrame v2.9.6 |
| K8s 客户端 | kubernetes-client Java SDK | client-go v0.34.1 |
| 配置解析 | Spring `@Value` | 环境变量 |
| 序列化 | Jackson | GoFrame + `encoding/json` |
| 静态资源 | jar 内 `classpath:/static/` | 文件系统目录 |
| 打包产物 | `higress-console.jar` | 静态链接二进制 `bin/console` |

## 4. 目录结构

```text
console/
├── main.go                       # 入口：参数解析、依赖组装、启动 HTTP 服务
├── internal/
│   ├── consts/                   # 常量（K8s 资源、插件、系统配置等）
│   ├── controller/               # HTTP 控制器：路由注册、中间件、请求处理
│   ├── errs/                     # 业务错误（对齐 Java BusinessException/AuthException）
│   ├── kubernetes/               # client-go 封装：Client、CRD 模型、配置
│   ├── model/                    # 领域模型与 DTO
│   ├── sdk/                      # 资源 CRUD 与转换逻辑（对应 Java SDK 服务层）
│   ├── service/                  # 高层服务：Session/Config/System/Dashboard 等
│   └── util/                     # AES、证书、随机串等工具
└── resource/
    ├── dashboard/                # 内置 Dashboard 配置（embed）
    ├── landing/                  # landing 页（embed）
    └── plugins/                  # 内置 Wasm 插件规格（embed）
```

## 5. 已实现接口清单

### 5.1 基础接口

| 方法 | 路径 | 说明 | 匿名 |
|------|------|------|------|
| POST | `/session/login` | 登录 | 是 |
| GET | `/session/logout` | 登出 | 是 |
| GET | `/healthz/ready` | 就绪探针 | 是 |
| ALL | `/landing` | Landing 页 | 是 |
| POST | `/system/init` | 系统初始化 | 是 |
| GET | `/system/info` | 系统信息/版本 | 是 |
| GET | `/system/config` | 系统配置 | 是 |
| GET | `/system/higress-config` | 读取 higress-config | 否 |
| PUT | `/system/higress-config` | 更新 higress-config | 否 |
| GET | `/user/info` | 当前用户信息 | 否 |
| POST | `/user/changePassword` | 修改密码 | 否 |
| GET | `/dashboard/init` | 初始化 Dashboard | 否 |
| GET/PUT | `/dashboard/info` | 读取/设置 Dashboard 地址 | 否 |
| GET | `/dashboard/configData` | Dashboard 配置数据 | 否 |
| ALL | `/grafana`、`/grafana/*` | Grafana 转发 | 否 |
| ALL | `/aiproxy`、`/aiproxy/*` | AI Proxy 转发 | 否 |

### 5.2 业务 API（`/v1/*`）

| 资源 | 路径 | 方法 |
|------|------|------|
| 路由 | `/v1/routes`、`/v1/routes/:name` | GET/POST/PUT/DELETE |
| 域名 | `/v1/domains`、`/v1/domains/:name`、`/v1/domains/:name/routes` | GET/POST/PUT/DELETE |
| 服务 | `/v1/services` | GET |
| 服务来源 | `/v1/service-sources`、`/v1/service-sources/:name` | GET/POST/PUT/DELETE |
| TLS 证书 | `/v1/tls-certificates`、`/v1/tls-certificates/:name` | GET/POST/PUT/DELETE |
| 消费者 | `/v1/consumers`、`/v1/consumers/:name` | GET/POST/PUT/DELETE |
| 代理服务器 | `/v1/proxy-servers`、`/v1/proxy-servers/:name` | GET/POST/PUT/DELETE |
| Wasm 插件 | `/v1/wasm-plugins`、`/v1/wasm-plugins/:name`、`/v1/wasm-plugins/:name/config`、`/v1/wasm-plugins/:name/readme` | GET/POST/PUT/DELETE |
| 插件实例（全局） | `/v1/global/plugin-instances`、`/v1/global/plugin-instances/:name` | GET/PUT/DELETE |
| 插件实例（域名） | `/v1/domains/:domainName/plugin-instances`、`.../:name` | GET/PUT/DELETE |
| 插件实例（路由） | `/v1/routes/:routeName/plugin-instances`、`.../:name` | GET/PUT/DELETE |
| 插件实例（服务） | `/v1/services/:serviceName/plugin-instances`、`.../:name` | GET/PUT/DELETE |
| AI 路由 | `/v1/ai/routes`、`/v1/ai/routes/:name` | GET/POST/PUT/DELETE |
| LLM Provider | `/v1/ai/providers`、`/v1/ai/providers/:name` | GET/POST/PUT/DELETE |

### 5.3 静态资源

- `GET /` 与 `/*any`：服务前端静态资源，未命中且非 API/静态前缀时回退到 `index.html`（SPA）。

## 6. 关键实现说明

### 6.1 路由与中间件

- 路由统一在 `internal/controller/router.go` 注册。
- GoFrame 路由参数：`:name` 为单段参数，`*any` 为模糊匹配。
- 中间件：`RecoverMiddleware`（panic → 统一错误响应）+ `AuthMiddleware`（登录校验）。
- `AuthMiddleware` 通过 `isAnonymousRequest` 识别匿名请求：
  - 非后端 API 路径（静态资源、`.html` 页面、SPA 路由）一律匿名。
  - `/session/*`、`/healthz/*`、`/landing`、`/system/init`、`/system/info`、`/system/config` 匿名。

### 6.2 K8s 客户端

- 统一使用 Go `client-go`，封装在 `internal/kubernetes`。
- 支持 Ingress/WasmPlugin/McpBridge 等 CRD 的读写与版本判断。

### 6.3 静态资源服务

- 从文件系统读取，不再 `go:embed` 前端构建产物。
- 路径优先级：命令行 `-static-dir` > 环境变量 `STATIC_DIR` > 可执行文件所在目录。
- `resolveStaticPath` 做路径清洗与穿越防护（禁止 `../`）。

### 6.4 版本号查询

- 编译期注入版本号变量：`var version = "unknown"`。
- 启动参数 `-v` / `-version` 打印版本号并退出。
- 通过 `-ldflags "-X main.version=<版本>"` 注入，用于区分二进制版本。

## 7. 构建与部署

### 7.1 构建

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w -X main.version=v1.2.3" -o bin/console .
```

产物为静态链接二进制（含 `resource/dashboard`、`resource/landing`、`resource/plugins` 内置资源）。

### 7.2 运行

```bash
# 默认监听 8080，端口可用 SERVER_PORT 覆盖
SERVER_PORT=8080 ./bin/console

# 指定前端静态资源目录（默认与可执行文件同目录）
./bin/console -static-dir /app/static

# 查询版本号
./bin/console -v
```

环境变量：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SERVER_PORT` | HTTP 监听端口 | `8080` |
| `STATIC_DIR` | 前端静态资源目录 | 可执行文件所在目录 |

## 8. 迁移过程中修复的关键问题

| 问题 | 根因 | 修复 |
|------|------|------|
| 编译报错 `*gvar.Var` 不能当 string | GoFrame `r.GetQuery()` 返回 `*gvar.Var` | 统一加 `.String()` |
| `/` 返回 not found | 缺少前端静态资源服务与 SPA fallback | 增加 `serveStatic` + `/*any` 路由 |
| 访问 `/` 报 `Login required` | AuthMiddleware 未放行静态资源/前端路由 | 非 API 路径匿名放行 |
| 前端报 `R.properties is undefined` | `encoding/json` 的 `omitempty` 会省略空 map，导致 `properties` 字段缺失 | 去掉 `ServiceSource.Properties` 的 `omitempty`，对齐 Jackson 输出空对象 |
| WasmPlugin CR 创建失败 | CR 缺少 `apiVersion`/`kind` | `wasmPluginToCr` 中显式填充 |
| WasmPlugin 更新错误信息不明确 | 原始错误被吞掉 | 在错误信息中追加 `err.Error()` |

## 9. 已知限制与后续工作

- `validateAndCleanUpConfigurations` 目前为空实现（TODO），未完成 WasmPlugin 配置校验与清理逻辑。
- 服务来源 MCP 配置等校验逻辑需与 Java SDK 进一步对齐。
- 前端静态资源需在部署时放置到指定目录，未随二进制内置。
