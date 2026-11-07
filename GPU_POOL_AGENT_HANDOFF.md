# GPU-Pool 项目交接与 AI 开发任务说明

> 面向 VS Code 中的 AI Coding Agent（Codex、GitHub Copilot Agent、OpenCode 等）  
> 更新时间：2026-11-07  
> 项目：**GPU-Pool / Friends GPU Pool**  
> 仓库：`https://github.com/Yzgll/GPU-Pool`  
> 本地工作区：`D:\Desktop\GPU-Pool`

## 0. 给 AI 开发 Agent 的执行指令（先读）

你正在接手一个**已经有实际代码和运行成果**的 Go + React 私有 GPU 算力共享平台。**不要重建项目、不要凭本文假设当前文件名或路由结构，也不要直接覆盖现有功能。**

1. 首先阅读仓库根目录 `README.md`、`docs/DEVELOPMENT.md`、`server/`、`agent/`、`frontend/`、`deploy/`，确认当前实现、启动方式、API、数据库迁移及 Git 工作区状态。
2. 以**仓库实际代码为准**；本文的“已完成”代表此前开发或验证的记录，不等于当前分支一定包含对应提交。如有差异，先报告差异。
3. 在已有 Coordinator、Agent、数据库、GPU 监控与算力池代码基础上增量开发，保持向后兼容。不要擅自重构整个项目。
4. 本轮最优先任务是：**为 GPU-Pool 提供 OpenAI 兼容 API，然后实现 Web Chat，并验证外部 CLI 编程 Agent 可接入**。
5. 每个阶段先给出简短实现方案与预计修改文件，再实施；完成后运行相关测试、构建并给出实际测试结果。不能把“代码写完”当成“验证通过”。
6. 不要在仓库中提交密码、API Key、`.env` 中的真实密钥、模型权重、数据库数据、生成的二进制文件或个人环境配置。
7. 现阶段以 Windows 本机单 GPU 作为**最小可运行验证环境**；多节点、远程 Agent、单模型跨机器切分是后续里程碑，不能假装已经实现。

---

## 1. 项目定位与最终目标

**GPU-Pool 是私有 GPU 算力共享与 AI 模型服务平台**，不是单纯的 GPU 监控面板。

最终要支持：

- 将多台个人/私有 GPU 主机注册为算力节点，中心统一发现、监控、管理与调度。
- 由节点 Agent 上报 GPU 硬件、显存、利用率、温度、心跳和模型服务能力。
- 用户在 Web 上查看节点、GPU、算力池、历史监控曲线，并直接与部署的 AI 模型聊天。
- 对外暴露**OpenAI 兼容 API**，让标准 SDK、Web 客户端和第三方 AI 工具通过统一入口调用模型。
- 用户可在本地安装 CLI / IDE 编程 Agent（如 OpenCode、Aider、Continue 等），使用 GPU-Pool 提供的模型完成代码理解、文件修改、测试与调试。**文件访问和命令执行主要由用户本机的客户端负责；远端 GPU 提供推理。**
- 多个节点提供不同模型实例时，Coordinator 根据可用性、资源、负载、模型能力调度推理请求。
- **长期核心目标：单个模型可跨多台物理机器、多张 GPU 切分部署和推理**。这与简单地将独立请求分发到不同 GPU 是两回事，需要专门的分布式推理引擎与通信设计。

### 目标使用方式

**Web：** 打开 GPU-Pool → 选择模型 → 多轮对话 → 流式查看回复。  
**API：** 客户端配置 `base_url=https://<gpu-pool-host>/v1`、API Key、模型 ID → 访问兼容接口。  
**CLI / IDE：** 安装编程 Agent → 配置 GPU-Pool API → 在本地项目目录内读取/修改代码、运行测试，模型推理由 GPU-Pool 处理。

> 术语：这里的目标是 **OpenAI-compatible API（兼容 OpenAI API）**，不要把它误写成“OpenAPI 规范”本身。OpenAPI/Swagger 文档可以另外提供，但不是本阶段的核心交付物。

---

## 2. 已知开发环境与基础设施

| 项目           | 当前已知情况                                                                                           |
| -------------- | ------------------------------------------------------------------------------------------------------ |
| 开发系统       | Windows 11（用户环境）；PowerShell、VS Code                                                            |
| 项目路径       | `D:\Desktop\GPU-Pool`                                                                                  |
| GitHub         | `Yzgll/GPU-Pool`                                                                                       |
| Git 网络       | 曾通过 `ssh://git@ssh.github.com:443/Yzgll/GPU-Pool.git` 推送成功；实际 remote 以 `git remote -v` 为准 |
| Go             | 1.24.0（历史环境记录）                                                                                 |
| Docker Desktop | 4.97.0 / Engine 29.9.2（历史环境记录）                                                                 |
| WSL2           | Ubuntu 20.04 及另一 Ubuntu 发行版（历史环境记录）                                                      |
| GPU            | NVIDIA RTX 3050 Laptop GPU，4GB 显存                                                                   |
| 驱动/CUDA      | NVIDIA 驱动 572.83；驱动报告 CUDA 12.8（历史记录）                                                     |
| 数据库         | PostgreSQL 18 Docker 容器，曾处于 healthy 状态                                                         |
| Coordinator    | 本地服务曾运行于 `http://127.0.0.1:8080`                                                               |
| 前端           | Vite 开发服务曾运行于 `http://localhost:5173`                                                          |
| Ollama         | Windows 已安装，命令行可用                                                                             |
| 模型           | `qwen2.5:1.5b`，`ollama list` 显示约 986 MB                                                            |
| 模型存储       | `E:\Ollama\models`，用户级 `OLLAMA_MODELS` 已设置                                                      |
| Ollama API     | `http://127.0.0.1:11434`                                                                               |

**注意：** 上述端口、版本、进程状态属于历史运行记录。开发 Agent 应检查实际端口、配置与进程，不能假定服务一直运行。

---

## 3. 已完成或曾验证的工作

### 3.1 项目骨架、数据库与中心服务

- 已创建仓库，基础目录包含 `server/`、`agent/`、`frontend/`、`inference/`、`deploy/`、`docs/`（以当前仓库为准）。
- 曾提交 `.gitignore`、`README.md`、`docs/DEVELOPMENT.md`。
- Go Coordinator 曾通过 `pgxpool` 连接 PostgreSQL，执行数据库迁移。
- 初始迁移 `001_init` 曾成功应用；已知表包括 `nodes`、`gpu_devices`、`resource_snapshots`、`schema_migrations`。

### 3.2 Agent 与 GPU 指标

- Go Agent 使用 `nvidia-smi` CSV 获取 GPU UUID、名称、显存总量/已用/剩余、利用率、温度、功率、驱动版本等。
- 相关解析单元测试曾覆盖单卡、多卡、`N/A`、空输出和异常情况，并通过。
- 已开发节点注册/心跳/GPU 资源上报链路；曾在本机验证。

### 3.3 Web 管理与监控

- 已开发节点/算力池 UI、GPU 监控页面（需以代码确认具体实现）。
- GPU 监控曾实现单 GPU 历史曲线。
- `frontend/src/pages/GPUMonitor.tsx` 曾同时渲染：
  - `GPUClusterHistoryChart`：所有 GPU 的平均利用率、总显存使用量、平均温度，时间范围 1h/6h/24h；
  - `GPUAllDevicesHistoryChart`：自动显示所有 GPU 的各自历史曲线，无需手工勾选。
- 对应后端历史接口曾包含：
  - `GET /api/v1/gpus/history/summary?range=1h|6h|24h`
  - `GET /api/v1/gpus/history/compare`
- 此前后端 `go test ./...` 曾通过，历史 API 曾有实际响应验证；**本轮需要重新验证当前仓库**。

### 3.4 本地模型部署与推理（2026-11-07 已实际验证）

- Ollama 安装成功，`ollama list` 显示 `qwen2.5:1.5b`，模型 ID `65ec06548149`。
- `ollama run qwen2.5:1.5b "你好"` 返回正常中文回答。
- `ollama ps` 曾显示 `PROCESSOR: 100% GPU`、运行时占用约 1.2 GB、上下文 4096；**100% GPU 指模型放置/处理器分配，不等于实时 GPU 利用率 100%**。
- 模型已从 C 盘默认目录复制至 `E:\Ollama\models`。用户将 C 盘旧 `models` 重命名后，模型仍能列出并正常推理；随后已删除旧备份目录。
- 已通过 PowerShell 成功请求 Ollama 的 `POST http://127.0.0.1:11434/v1/chat/completions`，返回了生成内容；**但 Windows PowerShell 5.1 的 `Invoke-RestMethod` 显示中文乱码，UTF-8 解码修复尚未获得用户验证**。
- **尚未证实 GPU-Pool Coordinator 自身实现了 `/v1/models` 或 `/v1/chat/completions`**，不能将 Ollama 自带接口误认为 GPU-Pool 已完成接口。

### 3.5 Git 状态注意事项

此前曾建议将 GPU 监控相关代码提交并推送，但没有看到最终 `git push` 成功输出。Agent 应首先执行 `git status`、`git log -5 --oneline`、`git remote -v`，确认当前分支和未提交修改；不要自行认定所有历史功能已在远端仓库。

---

## 4. 目标系统架构

```text
                  +------------------------------+
                  |  Web Chat / OpenCode / IDE   |
                  |  OpenAI SDK / Other Clients  |
                  +---------------+--------------+
                                  |
                      OpenAI-compatible HTTPS
                                  |
                  +---------------v--------------+
                  |       GPU-Pool API Gateway    |
                  | /v1/models                   |
                  | /v1/chat/completions         |
                  | API keys / streaming / errors|
                  +---------------+--------------+
                                  |
                  +---------------v--------------+
                  |        Coordinator           |
                  | models / nodes / routing     |
                  | scheduling / accounting      |
                  +---------------+--------------+
                                  |
                  +---------------v--------------+
                  |     Agent / Inference Layer  |
                  +----------+----------+--------+
                             |          |
                        GPU Node A  GPU Node B ...
                        Ollama etc. Ollama etc.
```

**阶段性实现策略：**

1. **M1 单机兼容网关：** Coordinator 先作为本地 Ollama 的兼容 API 入口，确保请求、SSE 和错误处理正确；这是联调过渡架构，不是最终多节点架构。
2. **M2 Web Chat：** 浏览器调用 Coordinator，完成多轮会话与流式显示。
3. **M3 CLI/IDE：** 用真实第三方客户端接入 GPU-Pool 网关，检验工具调用协议和代码模型能力。
4. **M4 多节点推理：** Coordinator 按模型/节点能力调度；Agent 在其所在机器启动/管理模型后端，使用受控、安全的通信链路。不能默认 Coordinator 可以直接访问每个 Agent 的 `127.0.0.1:11434`。
5. **M5 分布式单模型：** 单独设计模型分片、跨机通信、拓扑、显存规划、容错与性能测试；不能通过“显存相加”宣称实现。

---

## 5. 近期开发计划与明确验收标准

### M1：Coordinator OpenAI 兼容 API（**立即开始，最高优先级**）

**实施前：** 阅读现有 Go HTTP 路由、中间件、配置、错误处理和测试模式，选择最小改动方案。不要假定项目使用 Gin、Chi 或标准 `net/http`。

**功能要求：**

- `GET /v1/models`：返回符合 OpenAI 常见格式的 `object: "list"` 和 `data[]`（模型 ID、object 等）。第一版可从 Ollama `/api/tags` 或其兼容接口获取，但不能硬编码唯一模型作为长期实现。
- `POST /v1/chat/completions`：支持 `model`、`messages`、`stream`、常见采样参数；保留 `system` / `user` / `assistant` / `tool` 等角色和多轮消息语义。
- `stream: false`：返回 OpenAI 风格 JSON（`id`、`object`、`created`、`model`、`choices`、`usage` 等，尽可能符合实际后端能力）。
- `stream: true`：按 SSE 逐块返回 `chat.completion.chunk`，保持增量内容、结束事件 `data: [DONE]`、正确的 `Content-Type: text/event-stream`；请求取消时及时取消上游推理。不得把整个响应缓冲后伪装成流式。
- `tools`、`tool_choice`、`tool_calls`：为未来 Coding Agent 设计**透明且兼容的传递机制**；不要悄悄丢弃不支持的字段。应明确记录当前 Ollama/模型支持程度；对不支持的组合返回清晰错误，而非虚构工具调用。
- 后端地址由配置项控制，例如 `OLLAMA_BASE_URL=http://127.0.0.1:11434`；超时、错误映射、模型不存在等情况应可诊断。
- 设计 API Key 认证接口及配置方案：**本地开发可明确启用受限的开发模式，远程/生产环境必须验证密钥**；不得默认将无鉴权 API 暴露到公网。
- 不破坏已有 `/api/v1/...` 资源管理 API。
- 为 JSON 非流式、SSE、错误响应、上游不可达、取消请求、模型列表增加自动化测试（优先使用模拟 Ollama HTTP 服务）。

**验收：**

```powershell
# 1. 模型列表由 GPU-Pool 提供，而非直接访问 Ollama
Invoke-RestMethod http://127.0.0.1:8080/v1/models

# 2. 非流式聊天（若启用 API Key，应添加 Authorization Bearer 头）
$body = @{
  model = 'qwen2.5:1.5b'
  messages = @(@{ role = 'user'; content = '你好' })
  stream = $false
} | ConvertTo-Json -Depth 10
Invoke-RestMethod -Uri 'http://127.0.0.1:8080/v1/chat/completions' `
  -Method Post -ContentType 'application/json; charset=utf-8' `
  -Body ([Text.Encoding]::UTF8.GetBytes($body))

# 3. SSE 用 curl.exe -N 测试（注意 Windows PowerShell 中 curl 可能是别名）
# 按实际认证设置和请求体构造测试命令。
```

**中文乱码：** Windows PowerShell 5.1 对缺少 charset 的响应可能按错误编码解码。优先确保服务端返回标准 UTF-8 JSON 和适当 `Content-Type`；客户端可使用 `curl.exe` 或显式 UTF-8 解码验证。不要错误地把终端乱码归咎于模型。

### M2：Web AI Chat（M1 验收后）

- 在现有 React 前端增加聊天页面与导航，不影响 GPU 监控页。
- 支持模型列表、模型切换、多轮消息、流式逐字/逐块显示、停止生成、加载/错误状态。
- 第一版可采用会话内存态；持久化聊天记录属于后续增量功能，必须明确数据保存策略。
- Web 调用 GPU-Pool Coordinator，**不直接连接本地 Ollama**；妥善处理鉴权、跨域、反向代理和错误信息。
- UI 应支持 Markdown 与代码块显示，避免把模型输出作为不可信 HTML 直接执行。

**验收：** 从 Web 选择 `qwen2.5:1.5b`，输入中文问题，看到流式中文回复；连续追问保持上下文；点击停止可中断请求。

### M3：第三方 CLI / IDE 编程 Agent 接入

- 优先选用一个支持自定义 OpenAI-compatible endpoint 的现有编程 Agent 做端到端联调；例如 OpenCode、Aider 或 Continue，实际版本和配置格式以官方文档为准。
- 验证客户端可发现模型、发送多轮请求、获得流式回复、执行工具调用并正确处理返回。
- 编程 Agent 读取本地工作区、修改文件、执行 shell 命令需要**用户权限确认、工作区边界、危险操作保护**。
- **当前 `qwen2.5:1.5b` 仅用于协议联调，不应以此推断已具备 Codex 级别的代码代理能力。** 更强的代码模型、足够的上下文与稳定的工具调用能力是实际生产使用的前提。
- 后期可以开发原生 `gpu-pool` CLI，但不是 M1 的前置条件。

**验收：** 外部客户端通过 `http://127.0.0.1:8080/v1`（或受控远程 HTTPS 地址）成功连接，并在用户授权下完成一个小型代码编辑与测试任务。

### M4：多节点推理调度与服务治理

- Agent 上报节点支持的模型、运行时、显存占用、服务健康、推理并发与负载。
- Coordinator 根据模型可用性、显存容量、节点健康、负载、资源池选择后端。
- 请求超时、排队、并发限流、错误重试、取消与流式代理；注意推理中的请求不能随意重试到另一节点导致重复输出。
- Agent 与 Coordinator 之间采用鉴权、受控通信；不要直接开放所有节点的 Ollama 端口到公网。
- 明确模型安装/下载策略、磁盘空间管理、节点异构硬件和离线情况。

### M5：单模型跨多机切分（长期独立研发方向）

- 调研可跨节点部署的推理引擎和 tensor/pipeline parallel 能力、模型支持矩阵与互联网络要求。
- 明确跨机通信开销、带宽/延迟、显存分片、KV cache、调度、故障恢复与吞吐/时延指标。
- 先在两台真实机器上完成可复现实验，再接入 GPU-Pool 的统一 API 与资源调度。
- **禁止把多个独立 Ollama 实例的显存容量相加称作“单模型跨多机”。**

---

## 6. 推荐代码组织原则（不是强制现有路径）

在阅读仓库后，尽量复用现有结构，可考虑将职责拆分为：

- **OpenAI HTTP handlers**：协议解析、参数校验、JSON/SSE 输出。
- **Inference provider adapter**：Ollama 上游请求与流式转换，后续可扩展其他引擎。
- **Model catalog**：对外模型 ID、后端模型映射、能力信息。
- **Routing/scheduling service**：初期单机直连；未来由 Agent/节点调度提供目标后端。
- **Authentication & limits**：API Key、访问控制、配额、审计（逐步实现）。
- **Web chat**：独立页面/组件与 API 客户端，复用现有布局、样式与状态管理。

不要仅为了匹配上述名称创建重复目录。根据项目已有模块划分确定最终文件位置。

---

## 7. 安全与质量边界

1. `127.0.0.1:11434` 是本机 Ollama 服务地址，不是远程节点可直接访问的统一地址。
2. API Gateway 未来可能面向多用户；必须考虑 API Key、TLS、CORS、访问日志、速率限制与资源配额。初期可以分阶段交付，但不要把未实现的安全功能标记为已完成。
3. 模型生成的文本和工具调用参数都不可信。执行文件修改/命令的主体是**本地编程 Agent**，需受权限控制；GPU-Pool 服务端不应默认执行模型建议的任意命令。
4. 不把 `.ollama` 模型文件、用户代码仓库、聊天内容或密钥无授权上传到远程节点。
5. 对现有节点心跳、GPU 监控、算力池功能做回归验证。
6. 明确 Windows PowerShell 与 Linux shell 的命令差异；给用户的操作命令优先使用 PowerShell。
7. 性能和功能要有实际测试证据；“能聊天”不代表“能自动改代码”，“多个节点”不代表“单模型分布式”。

---

## 8. AI Agent 接手后的第一轮具体操作

**第一步：仓库审计（只读）。**

```powershell
cd D:\Desktop\GPU-Pool
git status
git log -5 --oneline
git remote -v
Get-ChildItem
Get-ChildItem server -Recurse -Filter *.go | Select-Object -ExpandProperty FullName
Get-ChildItem frontend -Depth 2
```

阅读现有启动文档、路由入口、数据库迁移和前端页面，输出**当前实际实现与本文的差异清单**。

**第二步：确认本地依赖。**

```powershell
ollama list
ollama ps
# 视项目实际配置确认 PostgreSQL、Coordinator 与前端是否在运行。
```

**第三步：制定 M1 最小变更计划。** 说明要改的文件、配置项、API 兼容范围、测试方案和对既有接口的影响。

**第四步：实现 M1。** 优先完成 `/v1/models`、`/v1/chat/completions` 非流式，再完成 SSE、错误处理与工具调用字段兼容；同步增加自动化测试。

**第五步：运行验证。** 运行后端测试、前端构建（若有改动）、通过 GPU-Pool 的 8080 端口实际请求 Ollama；记录成功输出及已知限制。

**第六步：提交。** 先展示 `git diff --stat`、测试结果与建议提交信息；由用户决定是否提交和推送。不要未经确认执行破坏性 Git 操作。

---

## 9. 建议的阶段完成定义（Definition of Done）

M1 只有满足以下条件才算完成：

- [ ] GPU-Pool 自身的 `/v1/models` 返回可用模型列表。
- [ ] GPU-Pool 自身的 `/v1/chat/completions` 能非流式返回中文回答。
- [ ] `stream: true` 可实时收到符合约定的 SSE chunk 与 `[DONE]`。
- [ ] 模型不存在、Ollama 不可达、请求格式错误均有清晰 HTTP 错误响应。
- [ ] 客户端中断时上游推理能够取消。
- [ ] 对工具调用字段的支持范围与限制已测试并记录。
- [ ] API Key/开发模式行为有明确文档，不会意外无鉴权对公网开放。
- [ ] 现有 GPU 监控、节点管理、算力池接口未回归失败。
- [ ] 后端自动化测试通过；相关代码可构建。
- [ ] `docs/` 中补充启动说明、环境变量、PowerShell API 示例和已知限制。

M2 完成后再进入 M3；M3 通过真实 CLI 客户端验证后，再优先建设多节点模型路由。

---

## 10. 交接给 AI Agent 的第一条任务指令（可直接复制）

> 请先阅读仓库根目录的 `GPU_POOL_AGENT_HANDOFF.md`，并结合当前仓库代码确认实际进度。不要重新搭建项目，不要破坏已完成的节点管理、算力池和 GPU 监控功能。先只读检查现有 Go Coordinator 路由、配置、测试和 Ollama 适配能力，向我报告“已存在的实现、缺口、拟修改文件及 M1 开发计划”。然后在我确认后，增量实现 GPU-Pool 的 OpenAI 兼容 API：`GET /v1/models` 和 `POST /v1/chat/completions`，支持非流式、SSE 流式、多轮消息、必要的错误处理和工具调用兼容字段，并用本机 `qwen2.5:1.5b` 实际验证。完成后提供测试结果、调用示例和后续 Web Chat 计划。以仓库真实代码为准，不要假设本文中的历史状态已经全部提交。
