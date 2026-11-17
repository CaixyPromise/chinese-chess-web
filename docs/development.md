# 开发与自托管

## 验证

在项目根目录启动开发依赖，构建并加载环境变量，操作顺序见 [README](../README.md)。

```sh
TEST_DATABASE_URL="$DATABASE_URL" make test
(cd web && npx playwright install chromium)
make test-e2e
```

数据库集成测试在开发库中创建隔离的 schema。浏览器测试需要运行中的应用和本地收件箱，会写入测试对局及账号。测试报告位于 `build/e2e/`。完成测试后退出应用进程，并运行 `docker compose stop` 停止开发依赖。

引擎格式由根目录的 `.clang-format` 配置，运行 `make format` 应用。引擎常量表随源码分发。

## 本地管理员

用页面注册并验证账号后，在已加载开发环境变量的终端运行：

```sh
./build/chess-admin grant --email player@example.test --reason 'Local administration'
```

按命令提示确认目标邮箱，再登录并打开 [管理工作台](http://127.0.0.1:8765/admin)。用 `revoke` 收回角色，授予和收回均记录审计。

## 自托管

应用由一个 Go 进程提供 HTTP、WebSocket 和静态资源。将 `build/chess-server`、`build/chess-admin`、`build/engine/xiangqi-engine`、`web/dist/` 和 `assets/` 按相对路径部署到目标机器。服务端和引擎需匹配目标操作系统与架构。使用专用服务账号从部署根目录运行 `./build/chess-server -root . -addr 127.0.0.1:8765`，由进程管理器负责自动重启。

| 环境变量 | 用途 |
| --- | --- |
| `DATABASE_URL` | 专用 PostgreSQL 账号和数据库连接 |
| `AUTH_PUBLIC_URL` | 用户访问的 HTTPS origin，用于校验来源及生成邮件链接 |
| `SMTP_ADDR` | 邮件提交地址，格式为 `host:port`。远端 SMTP 使用 STARTTLS |
| `SMTP_FROM` | 验证与重置邮件的发件地址 |
| `SMTP_USER`、`SMTP_PASSWORD` | SMTP 需要认证时使用 |
| `AUTH_TRUSTED_PROXY_IP` | 可选。只填写直接连接应用、且覆盖 `X-Real-IP` 的可信代理地址 |

[反向代理示例](../deploy/Caddyfile.example) 提供 HTTPS、WebSocket 转发及公网管理入口限制。将示例域名替换为你的域名，保持浏览器的 Origin，并将应用端口绑定到本机。使用示例中的本机 Caddy 时，设置 `AUTH_TRUSTED_PROXY_IP=127.0.0.1`，让服务按访客地址限流。管理访问通过受限网络上的 HTTPS 代理配置，并使用应用配置的同一 origin。部署后通过 `/health` 检查服务状态，再验证实际对局连接和邮件投递。

生产配置通过私密环境文件注入，使用专用数据库并在应用升级前备份。实际部署的容量、恢复和多实例协调按运行环境验证。
