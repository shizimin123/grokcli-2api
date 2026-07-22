# 项目排障记录

## 本地 Turnstile 过盾逻辑

本地过盾不是在本地计算 Turnstile token，也不调用付费打码服务。它在主容器内运行真实的 Camoufox/Chromium 浏览器，通过浏览器完成 Cloudflare Turnstile，然后将 token 返回给协议注册任务。

### 执行链路

1. 容器启动时运行 `turnstile-solver/api_solver.py`，默认只监听 `127.0.0.1:5072`。
2. 协议注册机访问 xAI 注册页，提取当前 Turnstile `sitekey`、Next.js action 等参数。
3. 注册机创建 `YesCaptchaSolver`，但 endpoint 指向本地 `http://127.0.0.1:5072`。
4. 注册机通过兼容 YesCaptcha 的接口提交并轮询任务：

   ```text
   POST /createTask
   POST /getTaskResult
   ```

5. 本地 solver 从浏览器池借出一个 Camoufox，创建独立 Browser Context，并应用当前注册任务选择的代理。
6. 浏览器打开 `accounts.x.ai/sign-up`，注入 Turnstile widget、关闭 Cookie 弹窗并模拟点击。
7. solver 持续读取 `input[name="cf-turnstile-response"]`。获得 token 后关闭当前 Context，将浏览器归还池中。
8. 注册机默认每 2 秒轮询一次，最长等待 120 秒。
9. 获得 Turnstile token 后才发送邮箱验证码，然后立即验证邮箱并创建账号。该顺序用于避免邮箱验证码在过盾期间过期。

### 并发与资源

默认相关配置：

```env
TURNSTILE_THREAD=3
TURNSTILE_BROWSER_TYPE=camoufox
TURNSTILE_LAZY=1
TURNSTILE_IDLE_SEC=180
```

- `TURNSTILE_THREAD` 是浏览器池槽位数，默认与协议注册并发一致。
- solver 默认通过 `TURNSTILE_THREAD_MAX=4` 限制最大浏览器线程数。
- 批量注册并发超过浏览器槽位时，任务在本地 semaphore 前排队，并定期更新等待心跳。
- `TURNSTILE_LAZY=1` 表示首次出现验证码任务时才初始化浏览器池。
- 连续 `TURNSTILE_IDLE_SEC` 秒没有验证码任务后，solver 自动关闭浏览器并释放内存。

### 代理行为

请求类型名称可能仍为 `TurnstileTaskProxyless`，这是为了兼容 YesCaptcha API，并不代表浏览器一定直连。

- 注册任务带有 `proxy` 时，本地 solver 会将其设置到 Browser Context。
- 代理 Context 创建失败时直接返回错误，不再静默回退直连。
- 注册任务没有代理时，本地浏览器才会直连。

### 健康状态

可以在容器内检查 solver：

```bash
curl -sS http://127.0.0.1:5072/health
```

关键字段：

- `pool_ready=true`：浏览器池已初始化。
- `thread`：配置的浏览器槽位数。
- `queue`：当前可借出的浏览器数。
- `owned`：solver 管理的浏览器总数。
- `in_flight`：正在处理的验证码任务数。
- `lazy=true`：启用按需初始化和空闲回收。

主要日志文件：

```text
/app/turnstile-solver/logs/turnstile_solver.log
/app/turnstile-solver/logs/registration_sidecar.log
```

关键实现位置：

- `entrypoint.sh`：启动本地 solver 和注册 sidecar。
- `grok2api/upstream/grok_build_adapter.py`：选择本地/云端过盾、等待 solver、排队及重试。
- `grok-build-auth/xconsole_client/solver.py`：YesCaptcha 兼容客户端和任务轮询。
- `turnstile-solver/api_solver.py`：浏览器池、Turnstile 注入、点击、token 提取和空闲回收。
