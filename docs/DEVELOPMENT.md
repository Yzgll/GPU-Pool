# Friends GPU Pool --- 项目开发总纲与实施计划

> 文档版本：v0.1.0\
> 创建日期：2026-11-01\
> 项目阶段：MVP / 第一阶段准备\
> 当前开发环境：Windows 11 + Go + Node.js + Git + Docker Desktop + WSL2\
> 推理引擎路线：vLLM（首选），分阶段验证单机和多机能力

------------------------------------------------------------------------

## 1. 项目定位

Friends GPU Pool 是一个面向朋友之间的私有 GPU
算力共享平台。每位参与者在自己的电脑上运行 Worker Agent，将本机 GPU
作为经过授权的共享算力资源接入平台。平台统一管理节点、资源、集群和任务，并在后续阶段将多个节点组织成分布式大模型推理服务。

它不是简单的多机多卡部署脚本，也不是把多台显卡的显存数字相加。平台需要同时解决：

-   节点身份、注册、心跳、在线状态和授权；
-   GPU / CPU / 内存 / 网络资源采集；
-   节点分组、资源调度和任务生命周期管理；
-   不同家庭网络之间的安全连接；
-   模型文件分发、模型切分和跨节点推理；
-   统一 Web 管理界面与 OpenAI 兼容 API。

### 1.1 项目最终目标

1.  朋友在 Windows 电脑安装客户端即可加入平台。
2.  节点自动注册、认证、心跳和上报资源。
3.  管理员能够查看节点在线状态、GPU
    型号、显存、利用率、温度、CPU、内存和网络。
4.  管理员能够创建逻辑算力集群，并将授权节点加入或移出集群。
5.  调度器能够根据 GPU 资源、网络质量和节点稳定性选择计算节点。
6.  后续能够使用 vLLM 将同一个模型部署到多个 GPU 节点，验证 Pipeline
    Parallelism 等分布式推理方式。
7.  所有人通过统一 Web 聊天页面或 OpenAI 兼容 API 使用模型。
8.  节点所有者始终能够控制是否共享、共享多少资源，以及何时退出。

### 1.2 当前已知硬件条件

  ------------------------------------------------------------------------------------------------
  角色 / 节点          GPU                               显存 用途
  -------------------- ---------------- --------------------- ------------------------------------
  项目发起人开发电脑   RTX 3050                           4GB Coordinator 开发、Agent 开发、GPU
                                                              采集验证；不作为主要大模型推理节点

  朋友节点             RTX 4060 Ti                       16GB 主要推理资源

  朋友节点             RTX 5060 Ti                       16GB 主要推理资源

  其他朋友节点         待登记                          待确认 接入后按实际硬件评估
  ------------------------------------------------------------------------------------------------

注意：

-   多台 GPU
    的物理显存可以分别承载模型权重分片，但不能当成一张统一的大显存 GPU。
-   可用显存必须扣除桌面程序、驱动、推理运行时、KV
    Cache、激活值和临时缓冲区。
-   RTX 3050 4GB
    适合开发资源采集与轻量测试，不应默认纳入正式模型分片集群。
-   RTX 4060 Ti 与 RTX 5060 Ti 属于不同 GPU
    架构。必须实测驱动、CUDA、PyTorch、vLLM、通信后端和算子兼容性，不预先承诺异构多机一定可用。

### 1.3 网络现状与假设

目前参与者都在成都，但"同城"不等于"同一局域网"。家庭宽带通常存在
NAT、动态公网地址、运营商限制和不同路由路径。

因此分阶段处理：

-   第一阶段：本机回环地址和同一局域网联调。
-   第二阶段：使用 WireGuard / Tailscale
    类组网方案，让异地家庭网络形成受控的虚拟网络。
-   第三阶段：测量节点间 RTT、抖动、吞吐、丢包和实际直连 / 中继路径。
-   第四阶段：只有网络测试达到要求，才尝试 vLLM 多机推理。
-   不把"成都同城"当作低延迟、高带宽的保证。

------------------------------------------------------------------------

## 2. 技术架构原则

### 2.1 控制平面中心化，计算平面分布式

**控制平面（Control Plane）**由 Coordinator 统一管理：

-   用户和节点身份；
-   节点注册、认证、心跳；
-   资源台账与监控；
-   逻辑集群；
-   任务创建、调度和状态；
-   模型部署元数据；
-   统一 API 入口和访问控制。

**计算平面（Data Plane）**由各个 Worker 执行：

-   本地 GPU 资源采集；
-   本地推理运行时管理；
-   模型文件缓存；
-   接收经过授权的任务；
-   执行 vLLM 或其他允许的计算任务；
-   向 Coordinator 汇报任务状态和指标；
-   在分布式推理时与其他 Worker 交换必要的数据。

中心服务器不转发所有 GPU
计算数据。模型推理的数据面应尽量由计算节点直接通信，避免中心端成为带宽瓶颈。

### 2.2 第一阶段避免过度微服务化

MVP 使用模块化单体 Coordinator：

-   一个 Go 服务；
-   一个 PostgreSQL 数据库；
-   一个 Vue 管理前端；
-   多个独立 Worker Agent；
-   WebSocket 用于实时状态 / 任务通知，REST API 用于资源管理。

第一阶段不引入 Kubernetes、Redis、Kafka、Ray
等非必要组件。后续确实出现规模或功能需求时再引入。

### 2.3 代码仓库采用 Monorepo

建议仓库：

``` text
friends-gpu-pool/
├── server/                  # Go Coordinator
│   ├── cmd/coordinator/
│   ├── internal/
│   │   ├── config/
│   │   ├── database/
│   │   ├── auth/
│   │   ├── node/
│   │   ├── gpu/
│   │   ├── cluster/
│   │   ├── scheduler/
│   │   ├── task/
│   │   └── api/
│   ├── migrations/
│   └── go.mod
├── agent/                   # Go Worker Agent
│   ├── cmd/agent/
│   ├── internal/
│   │   ├── config/
│   │   ├── identity/
│   │   ├── registration/
│   │   ├── heartbeat/
│   │   ├── monitor/
│   │   ├── gpu/
│   │   ├── executor/
│   │   └── runtime/
│   └── go.mod
├── frontend/                # Vue 3 + TypeScript
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── layouts/
│   │   └── views/
│   └── package.json
├── inference/               # 后续 vLLM / 分布式推理实验
│   ├── docker/
│   ├── configs/
│   ├── scripts/
│   └── docs/
├── deploy/
│   ├── docker-compose.yml
│   ├── env.example
│   └── scripts/
├── docs/
│   ├── DEVELOPMENT.md
│   ├── architecture.md
│   ├── api.md
│   ├── database.md
│   ├── security.md
│   └── runbook.md
├── .gitignore
└── README.md
```

建议 `server` 和 `agent` 使用独立 Go
Module；仓库级文档和部署文件统一管理。

------------------------------------------------------------------------

## 3. 技术选型

  --------------------------------------------------------------------------------------------
  层次                    选型                    第一阶段职责
  ----------------------- ----------------------- --------------------------------------------
  Coordinator             Go + Gin                REST API、节点管理、集群管理、任务管理

  Worker Agent            Go                      Windows
                                                  常驻客户端、资源采集、注册、心跳、任务执行

  数据库                  PostgreSQL              用户、节点、GPU、监控、集群、任务持久化

  数据库驱动              pgx                     Go 连接 PostgreSQL

  实时通信                WebSocket               实时通知、任务状态更新

  前端                    Vue 3 + TypeScript +    管理控制台
                          Vite                    

  UI                      Element Plus            后台管理组件

  图表                    ECharts                 GPU / 节点监控曲线

  GPU 采集                第一版 NVIDIA-SMI，后续 GPU 型号、显存、利用率、温度、功耗
                          NVML                    

  系统采集                gopsutil                CPU、内存、磁盘、网络

  容器                    Docker Desktop + WSL2   本机 PostgreSQL；后续 vLLM Linux 容器

  推理引擎                vLLM                    后续单机和多机模型推理

  多机运行时              vLLM 支持的 Ray /       后续验证，不进入第一阶段基础依赖
                          多机执行路径            

  组网                    WireGuard / Tailscale   第二阶段后异网互联
                          类方案                  

  版本管理                Git                     分支、提交、版本标签
  --------------------------------------------------------------------------------------------

### 3.1 Windows 与 vLLM 的关系

vLLM 官方主线不提供原生 Windows 运行环境。Windows 开发机使用 WSL2 /
Linux 容器运行 vLLM。当前 Docker Desktop 已安装并使用 WSL2
后端，后续需要单独验证 Docker 容器能否访问本机 NVIDIA GPU。

不要将 vLLM 与 Go Agent 混为一个进程：

-   Go Agent 负责节点身份、监控、任务和运行时生命周期。
-   vLLM 在独立 Linux 容器中运行。
-   Agent 通过受控的本地接口启动 /
    停止已定义的推理服务，并采集运行状态。
-   不允许 Coordinator 任意下发 PowerShell、Shell 或 Docker 命令。

### 3.2 vLLM 分布式路线

vLLM 是最终优先验证的推理引擎，但采用逐级验证：

1.  单节点、单 GPU vLLM：验证镜像、驱动、模型加载、OpenAI API。
2.  单机多 GPU（如果某位朋友有多卡）：验证 Tensor Parallel。
3.  两台同城节点：先建立 VPN / 直连网络，测试带宽和 RTT，再验证 vLLM
    多节点运行时。
4.  多节点模型切分：优先评估 Pipeline Parallelism；Tensor Parallelism
    会频繁进行 GPU 间通信，跨家庭网络通常更敏感。
5.  多节点故障恢复、模型重新部署和统一 API。

vLLM 官方多节点部署要求各节点具有一致的执行环境、模型路径和 Python
包版本。应优先用固定版本的容器镜像统一环境。实际是否支持当前混合 GPU
型号组合，必须以 PoC 测试为准。

### 3.3 模型选择原则

第一轮不直接追求"最大的模型"。选择模型时需要评估：

-   模型架构是否被当前 vLLM 版本支持；
-   权重精度 / 量化格式是否被支持；
-   模型权重大小；
-   KV Cache 预算；
-   每个节点可用显存；
-   分片边界和层数；
-   生成速度、首 token 延迟和吞吐；
-   许可证和模型分发权限。

------------------------------------------------------------------------

## 4. 当前开发环境基线

截至 2026-11-01，开发机检查结果：

  -----------------------------------------------------------------------
  组件                                当前状态
  ----------------------------------- -----------------------------------
  OS                                  Windows 11，Build 26200.9711

  GPU                                 NVIDIA GeForce RTX 3050 4GB

  NVIDIA Driver                       572.83

  `nvidia-smi` 显示 CUDA              12.8（驱动支持信息，不代表已安装
                                      CUDA Toolkit）

  Go                                  1.24.0 windows/amd64

  Node.js                             24.11.1

  npm                                 11.6.2

  Git                                 2.56.0.windows.4

  Docker Desktop                      4.97.0

  Docker Engine                       29.9.2

  Docker Compose                      v5.6.1

  WSL                                 WSL 2

  Ubuntu                              Ubuntu-20.04 和 Ubuntu
                                      两个发行版均存在；默认 Ubuntu-20.04

  Docker Context                      desktop-linux
  -----------------------------------------------------------------------

开发原则：

-   Go Coordinator 和 Agent 优先在 Windows 原生环境编译、运行和调试。
-   PostgreSQL 通过 Docker Desktop 运行。
-   vLLM 通过 Linux 容器 / WSL2 环境单独验证。
-   不因为已经安装 WSL 就把所有开发都迁入 Linux。
-   现阶段不要卸载两个 Ubuntu 发行版；先保持环境稳定。
-   WSL 出现 localhost 代理警告时，只有在 WSL
    内确实需要联网且失败时才处理，不影响 Windows 原生 Go 开发。

### 4.1 环境验收命令

在 Windows CMD / PowerShell：

``` powershell
go version
node -v
npm -v
git --version
docker version
docker compose version
wsl --status
wsl -l -v
nvidia-smi
```

Docker GPU 验证应作为 vLLM PoC 的独立任务执行，不能仅凭 Docker Client /
Server 正常就认定容器已能使用 GPU。

------------------------------------------------------------------------

## 5. 系统模块设计

### 5.1 Coordinator

Coordinator 是平台控制中心，第一阶段包含：

  模块        职责
  ----------- ------------------------------------
  Config      配置加载、环境变量、日志
  Database    PostgreSQL 连接池、迁移
  Auth        管理员登录、注册 Token、节点 Token
  Node        节点注册、查询、禁用、撤销
  GPU         GPU 设备清单和资源快照
  Metrics     监控历史数据写入与查询
  Cluster     逻辑集群创建、节点成员关系
  Scheduler   第一版基础资源筛选与节点选择
  Task        任务创建、下发、状态机、结果
  API         REST 路由、参数校验、错误响应
  Realtime    WebSocket 通知

第一阶段建议单体部署，内部按模块划分，不提前拆成微服务。

### 5.2 Worker Agent

Agent 是每台朋友电脑安装的客户端，第一阶段包含：

  模块              职责
  ----------------- ----------------------------------------
  Config            读取配置和本地身份
  Identity          首次注册后保存 node_id 和凭证
  Registration      使用一次性注册码加入平台
  Heartbeat         定时上报在线状态
  GPU Monitor       采集一台或多台 NVIDIA GPU 信息
  System Monitor    CPU、内存、磁盘、网络
  Executor          执行任务白名单中的任务
  Runtime Manager   后续管理 vLLM 容器，不在首个里程碑实现
  Logger            本地日志、错误信息、日志轮转

Agent 第一版作为命令行程序运行，先不做 Windows GUI。稳定后再做 Windows
Service / 安装包。

### 5.3 前端

第一阶段页面：

1.  Dashboard：节点总数、在线节点、GPU 数量、物理显存统计、任务数。
2.  节点管理：节点名称、所有者、操作系统、Agent 版本、在线状态。
3.  GPU 详情：型号、总显存、已用显存、利用率、温度、功耗。
4.  集群管理：创建集群、添加 / 移除节点、查看集群资源。
5.  任务管理：任务类型、目标节点、状态、执行结果。
6.  系统设置：注册码、访问密钥、基本配置。

页面上的显存总量必须明确标注为"物理显存合计"，不得展示为"单卡可用显存"。

------------------------------------------------------------------------

## 6. 数据库设计

第一阶段核心表：

### 6.1 users

  字段            类型建议      说明
  --------------- ------------- ----------------
  id              UUID          主键
  username        VARCHAR       用户名，唯一
  password_hash   TEXT          密码哈希
  role            VARCHAR       admin / member
  created_at      TIMESTAMPTZ   创建时间

### 6.2 registration_tokens

  字段         类型建议      说明
  ------------ ------------- -----------------
  id           UUID          主键
  token_hash   TEXT          注册 Token 哈希
  created_by   UUID          创建者
  max_uses     INT           最大使用次数
  used_count   INT           已使用次数
  expires_at   TIMESTAMPTZ   过期时间
  revoked_at   TIMESTAMPTZ   撤销时间

### 6.3 nodes

  字段                类型建议      说明
  ------------------- ------------- ---------------------------------------
  id                  UUID          节点 ID
  owner_id            UUID          所属用户
  hostname            TEXT          主机名
  os_type             TEXT          windows / linux
  os_version          TEXT          系统版本
  agent_version       TEXT          Agent 版本
  status              TEXT          online / suspect / offline / disabled
  last_heartbeat_at   TIMESTAMPTZ   最近心跳
  created_at          TIMESTAMPTZ   注册时间
  revoked_at          TIMESTAMPTZ   撤销时间

### 6.4 node_credentials

  字段         类型建议      说明
  ------------ ------------- -----------------
  id           UUID          主键
  node_id      UUID          节点 ID
  token_hash   TEXT          节点 Token 哈希
  created_at   TIMESTAMPTZ   创建时间
  revoked_at   TIMESTAMPTZ   撤销时间

明文 Node Token 只在签发时返回一次；服务端不保存明文。

### 6.5 gpus

  字段                 类型建议   说明
  -------------------- ---------- -----------------
  id                   UUID       主键
  node_id              UUID       所属节点
  gpu_uuid             TEXT       NVIDIA GPU UUID
  index                INT        本机 GPU 序号
  name                 TEXT       GPU 型号
  vram_total_mb        BIGINT     总显存
  driver_version       TEXT       驱动版本
  compute_capability   TEXT       可选，计算能力

对 `(node_id, gpu_uuid)` 建唯一约束。GPU 静态信息与动态指标分开保存。

### 6.6 node_metrics

保存采样指标：

-   node_id；
-   sampled_at；
-   cpu_usage_percent；
-   memory_total_mb；
-   memory_used_mb；
-   gpu_uuid；
-   gpu_utilization_percent；
-   gpu_memory_used_mb；
-   gpu_memory_free_mb；
-   temperature_c；
-   power_w；
-   network_rx_bytes；
-   network_tx_bytes。

按 30～60 秒采样写入历史数据；心跳可以每 5～10
秒一次，但不要每次心跳都产生大量历史记录。

### 6.7 clusters 与 cluster_nodes

`clusters` 保存集群元信息；`cluster_nodes` 保存集群和节点的多对多关系。

同一节点是否允许加入多个集群，由后续资源调度策略决定。MVP
可以先限制为一个活动计算集群，减少资源冲突。

### 6.8 tasks

任务字段：

-   id；
-   cluster_id；
-   node_id；
-   task_type；
-   payload_json；
-   status；
-   created_at；
-   dispatched_at；
-   started_at；
-   finished_at；
-   result_json；
-   error_message。

任务状态建议：

``` text
pending -> dispatched -> running -> succeeded
                                \-> failed
                                \-> cancelled
```

需要处理超时、重复上报和 Agent 重连后的任务状态恢复。

------------------------------------------------------------------------

## 7. API 设计

统一前缀：`/api/v1`

  方法     路径                               说明
  -------- ---------------------------------- ----------------------
  GET      `/health`                          服务健康检查
  POST     `/auth/login`                      管理员登录
  POST     `/registration-tokens`             创建节点注册 Token
  POST     `/nodes/register`                  Agent 注册
  POST     `/nodes/heartbeat`                 Agent 心跳与资源上报
  GET      `/nodes`                           查询节点列表
  GET      `/nodes/{id}`                      节点详情
  POST     `/nodes/{id}/disable`              禁用节点
  POST     `/nodes/{id}/revoke`               撤销节点凭证
  GET      `/nodes/{id}/metrics`              节点历史指标
  POST     `/clusters`                        创建集群
  GET      `/clusters`                        查询集群
  POST     `/clusters/{id}/nodes`             添加节点
  DELETE   `/clusters/{id}/nodes/{node_id}`   移除节点
  POST     `/tasks`                           创建任务
  GET      `/tasks`                           查询任务
  GET      `/tasks/{id}`                      任务详情

Agent 与 Coordinator 的通信第一版可使用 HTTPS REST
轮询任务，节点和任务状态通过 WebSocket
推送到前端。后续节点数量增加或任务实时性要求提高时，再把 Agent
任务通道升级为双向 WebSocket / gRPC。

所有节点 API 必须校验节点身份。不能只相信请求体里的 `node_id`。

------------------------------------------------------------------------

## 8. 分批开发计划

开发按里程碑推进。每一批必须完成代码、运行验证、测试记录和 Git
提交，再进入下一批。

### Phase 0：开发环境与仓库基线

目标：保证本机可稳定开发和运行。

任务：

-   [x] Windows 11 开发机确认。
-   [x] Go 安装并验证。
-   [x] Node.js / npm 安装并验证。
-   [x] NVIDIA 驱动和 `nvidia-smi` 验证。
-   [x] Git 安装并验证。
-   [x] WSL2 / Ubuntu 启动验证。
-   [x] Docker Desktop 安装并验证 Client / Server。
-   [ ] 创建项目根目录与 Git 仓库。
-   [ ] 创建 `server`、`agent`、`frontend`、`deploy`、`docs`。
-   [ ] 添加 `.gitignore`、README 和环境变量模板。
-   [ ] 第一次 Git commit。

验收：

``` powershell
go version
node -v
npm -v
git --version
docker version
docker compose version
nvidia-smi
```

### Phase 1：Worker GPU 采集最小程序

目标：不依赖数据库和 Coordinator，先让 Go Agent 在本机读出 GPU 信息。

任务：

1.  初始化 Agent Go Module。
2.  定义 GPU 结构体。
3.  调用 `nvidia-smi` 获取 GPU 列表。
4.  解析 CSV，不能只返回原始字符串。
5.  支持多 GPU。
6.  处理命令不存在、超时、驱动异常和字段缺失。
7.  输出 JSON。
8.  增加单元测试和样例数据。

建议采集字段：

-   index；
-   uuid；
-   name；
-   memory.total；
-   memory.used；
-   memory.free；
-   utilization.gpu；
-   temperature.gpu；
-   power.draw；
-   driver_version。

验收：

-   RTX 3050 能被识别；
-   JSON 字段类型正确；
-   多 GPU 输出为数组；
-   GPU 不可用时程序返回可诊断错误；
-   不因单次采集失败而崩溃。

里程碑：`M1 - GPU Monitor Prototype`

### Phase 2：Coordinator 基础服务

目标：启动 Go HTTP 服务并具备数据库连接。

任务：

1.  初始化 Server Go Module。
2.  建立 `cmd/coordinator` 入口。
3.  加载 YAML / 环境变量配置。
4.  配置结构化日志。
5.  建立 Gin 路由。
6.  实现 `/health`。
7.  Docker Compose 启动 PostgreSQL。
8.  使用 pgx 建立连接池。
9.  增加数据库迁移工具和第一版迁移文件。
10. 增加优雅退出和启动健康检查。

验收：

-   Coordinator 正常启动；
-   `/health` 返回 200；
-   数据库连接成功；
-   PostgreSQL 重启后服务可恢复；
-   数据库密码不提交 Git。

里程碑：`M2 - Coordinator + PostgreSQL`

### Phase 3：节点注册、认证与心跳

目标：本机 Agent 成功注册到 Coordinator，并持续上报资源。

任务：

1.  创建管理员和注册 Token。
2.  实现注册 Token 哈希存储、有效期和使用次数。
3.  实现节点注册接口。
4.  注册成功后签发节点专属 Token。
5.  Agent 本地保存 node_id 和凭证。
6.  实现节点 Token 哈希验证和撤销。
7.  实现心跳 API。
8.  Agent 定时上报系统和 GPU 资源。
9.  Coordinator 维护最后心跳时间。
10. 实现 Online / Suspect / Offline 状态计算。
11. Agent 断线重试、指数退避和重新认证。
12. 资源快照更新与历史指标采样分离。

建议初始规则：

-   心跳间隔：10 秒；
-   15 秒未收到：Suspect；
-   30 秒未收到：Offline。

阈值应可配置，避免因电脑休眠、网络短暂抖动误判。

验收：

-   本机 Agent 注册成功；
-   数据库有对应节点和 GPU 记录；
-   心跳持续更新；
-   停止 Agent 后节点变为 Offline；
-   重新启动 Agent 后恢复 Online；
-   撤销 Token 后旧凭证无法继续上报。

里程碑：`M3 - Node Registration & Heartbeat`

### Phase 4：节点管理与逻辑集群

目标：通过 API 管理节点和集群成员。

任务：

1.  节点列表和详情 API。
2.  节点禁用、撤销和软删除。
3.  GPU 当前资源查询。
4.  历史指标查询。
5.  集群创建、查询和删除。
6.  节点加入、移出集群。
7.  校验节点归属和共享授权。
8.  统计集群物理显存总量，并明确标注为物理资源合计。
9.  为节点增加共享开关和资源上限配置。

验收：

-   能创建集群；
-   能添加 / 移除节点；
-   禁用节点不能继续接收任务；
-   离线节点仍保留历史记录；
-   不允许未授权用户操作其他人的节点。

里程碑：`M4 - Node & Cluster Management`

### Phase 5：Vue 管理控制台

目标：浏览器中完成节点和集群管理。

任务：

1.  初始化 Vue 3 + TypeScript + Vite。
2.  安装 Element Plus、Axios、ECharts。
3.  建立登录页。
4.  建立 Dashboard。
5.  建立节点列表和详情页。
6.  建立 GPU 监控图表。
7.  建立集群管理页。
8.  建立任务列表页。
9.  WebSocket 实时刷新节点状态。
10. 增加加载、空状态、错误提示和权限控制。

验收：

-   登录后看到 Dashboard；
-   显示节点在线 / 离线；
-   显示 GPU 显存、利用率、温度；
-   集群操作能够真实写入数据库；
-   浏览器刷新后数据仍然存在。

里程碑：`M5 - Web Console`

### Phase 6：任务下发与执行框架

目标：实现受控的远程任务执行，而不是远程任意命令执行。

第一批白名单任务：

-   `gpu_info_check`：GPU 信息检查；
-   `system_info_check`：系统资源检查；
-   `runtime_health_check`：后续推理运行时健康检查。

任务需要：

-   task_id；
-   task_type；
-   目标节点；
-   参数 schema；
-   创建者；
-   超时时间；
-   状态；
-   结果；
-   错误原因。

Agent
只允许执行编译进程序或明确注册的任务处理器。禁止直接执行服务端传来的任意
Shell / PowerShell 字符串。

验收：

-   控制台创建任务；
-   Agent 收到任务；
-   Agent 执行白名单任务；
-   Coordinator 收到任务结果；
-   任务超时、失败、取消均有状态记录。

里程碑：`M6 - Task Execution MVP`

### Phase 7：同城多节点接入与网络验证

目标：至少接入 2～4 台真实朋友电脑，验证跨主机管理。

任务：

1.  为朋友生成一次性注册码。
2.  打包 Windows Agent。
3.  先在同一局域网测试。
4.  再测试不同家庭网络。
5.  评估 Tailscale / WireGuard 组网。
6.  测量 RTT、抖动、吞吐、丢包。
7.  检查直连或中继路径。
8.  测试节点休眠、断网、重启和退出共享。
9.  检查中心服务重启后的节点恢复。

验收：

-   至少 2 台外部电脑成功注册；
-   多节点同时在线；
-   资源信息互不串号；
-   网络断开后状态正确；
-   Agent 重连后恢复；
-   未授权节点无法加入。

里程碑：`M7 - Multi-node Pool`

### Phase 8：vLLM 单机推理 PoC

目标：先在一台 16GB GPU 节点上跑通 vLLM，不直接进入多机。

任务：

1.  确认该节点 NVIDIA 驱动和 GPU 型号。
2.  确认 Docker / WSL2 能访问 GPU。
3.  选择官方 vLLM Linux 容器镜像并固定版本。
4.  选择一个适合 16GB 显存的模型和量化格式。
5.  设置模型缓存目录。
6.  启动 vLLM OpenAI-compatible server。
7.  调用 `/v1/models`。
8.  调用 `/v1/chat/completions`。
9.  记录显存、首 token 延迟、生成速度和稳定性。
10. 记录模型版本、镜像版本、参数和测试结果。

验收：

-   vLLM 能识别目标 GPU；
-   模型能够加载；
-   OpenAI 兼容 API 正常；
-   多轮请求稳定；
-   资源监控能够观察推理占用。

注意：开发机 RTX 3050 4GB 只用于环境和轻量模型验证；主要 PoC
应使用朋友的 16GB GPU。

里程碑：`M8 - vLLM Single-node PoC`

### Phase 9：vLLM 多机推理 PoC

目标：验证一个模型能否跨多个朋友节点切分并推理。

前置条件：

-   Phase 7 多节点网络验证通过；
-   每台节点能运行兼容的 Linux / vLLM 容器环境；
-   模型和依赖版本一致；
-   模型分片、显存预算和通信路径已经评估；
-   有稳定的节点 IP / VPN 地址；
-   防火墙仅开放必要的节点通信端口。

实验顺序：

1.  两台同城节点先测试网络吞吐和 RTT。
2.  测试两台节点间基础 NCCL / 通信能力（以所选部署方式支持为准）。
3.  使用 vLLM 支持的多节点运行时和 Pipeline Parallelism 做 PoC。
4.  记录每个节点的显存占用、GPU 利用率和通信开销。
5.  与单节点基线对比首 token 延迟、tokens/s 和稳定性。
6.  模拟一个节点掉线，观察服务失败方式和恢复方式。
7.  再决定是否扩展到 3～4 台节点。

重要限制：

-   vLLM 多节点需要一致的运行环境、模型路径和 Python 依赖。
-   vLLM 的多机执行通常涉及 Ray 或其支持的多节点执行机制。
-   Pipeline Parallelism 与 Tensor Parallelism
    的通信模式不同；跨家庭网络应优先评估 Pipeline Parallelism。
-   不能假设异构 4060 Ti / 5060 Ti 的组合无需调试即可工作。
-   如果家庭网络延迟、带宽或丢包不合格，应停止多机推理尝试，先优化网络或改为单节点模型副本调度。
-   多机分布式推理的目标首先是"模型能正确运行"，其次才是性能优化。

里程碑：`M9 - vLLM Multi-node PoC`

### Phase 10：统一推理服务与平台化

目标：把通过验证的 vLLM 服务纳入平台管理。

任务：

1.  Model Manager 记录模型、版本、量化格式和部署配置。
2.  Agent 管理受控的 vLLM Runtime 生命周期。
3.  Coordinator 维护模型部署状态。
4.  API Gateway 统一鉴权和请求路由。
5.  Web Chat 页面。
6.  OpenAI 兼容 API。
7.  用户配额、并发限制和访问日志。
8.  节点贡献统计和共享额度。
9.  运行日志、告警和基本容错。

里程碑：`M10 - Friends GPU Pool Beta`

------------------------------------------------------------------------

## 9. 安全与资源共享原则

平台运行在朋友的个人电脑上，安全和授权是核心需求，不是后续可选功能。

### 9.1 节点所有者控制权

每位朋友应能：

-   开启 / 关闭共享；
-   设置最大可用显存比例；
-   设置允许共享的时间段（后续）；
-   设置是否允许模型推理任务；
-   查看当前运行任务；
-   随时停止共享和撤销节点身份。

### 9.2 凭证安全

-   注册 Token 应有有效期和使用次数限制。
-   Node Token 应每个节点独立。
-   数据库只保存 Token 哈希。
-   Agent 本地凭证使用 Windows Credential Manager / DPAPI 等安全存储。
-   支持凭证撤销和轮换。
-   管理员密码使用成熟密码哈希算法，不得明文保存。
-   生产环境 API 使用 TLS。

### 9.3 任务执行安全

-   禁止任意 Shell / PowerShell 命令执行。
-   任务类型使用白名单。
-   对任务参数做 schema 校验。
-   设置超时、资源限制和并发限制。
-   任务执行结果需要记录审计信息。
-   vLLM 容器使用固定镜像版本和明确的 GPU / 网络 / 文件挂载权限。
-   不向普通用户开放 Docker 管理权限。

### 9.4 网络安全

-   控制面和计算数据面分离。
-   节点通信使用加密隧道或 TLS。
-   不要求朋友直接开放任意公网端口。
-   中继流量需要单独测量成本和带宽瓶颈。
-   不将数据库端口暴露到公网。
-   仅开放必要服务端口。

------------------------------------------------------------------------

## 10. 测试策略与验收指标

### 10.1 单元测试

-   GPU 输出解析；
-   配置加载；
-   注册 Token 校验；
-   心跳状态计算；
-   任务状态机；
-   节点和集群权限。

### 10.2 集成测试

-   Agent → Coordinator 注册；
-   Agent → Coordinator 心跳；
-   Coordinator → PostgreSQL 写入；
-   前端 → API 查询；
-   任务下发与结果回传。

### 10.3 故障测试

-   Coordinator 重启；
-   PostgreSQL 重启；
-   Agent 进程退出；
-   节点断网；
-   节点休眠；
-   GPU 驱动不可用；
-   Token 被撤销；
-   任务超时；
-   节点所有者主动停止共享。

### 10.4 第一阶段核心验收

-   [ ] 本机 RTX 3050 能注册为节点。
-   [ ] GPU 采集输出结构化数据。
-   [ ] Coordinator 能持久化节点信息。
-   [ ] 心跳和在线状态正确。
-   [ ] 节点断线后自动标记 Offline。
-   [ ] 节点重连后恢复 Online。
-   [ ] Dashboard 展示 GPU 资源。
-   [ ] 能创建逻辑集群并添加节点。
-   [ ] 能创建和执行白名单测试任务。
-   [ ] 至少两台朋友电脑接入成功。
-   [ ] 不存在任意远程命令执行接口。
-   [ ] Node Token 可撤销。

------------------------------------------------------------------------

## 11. 开发协作约定

为了确保后续按步骤推进，统一以下开发规则：

1.  一次只推进一个明确任务，不同时铺开多个模块。
2.  每次开发先说明目标、涉及文件、接口和验收方法。
3.  代码按文件给出完整内容，标明准确路径。
4.  每个模块完成后先编译、运行和测试。
5.  测试不通过先定位问题，不带着错误进入下一阶段。
6.  API、数据库字段和配置发生变化时，同步更新文档。
7.  每个里程碑完成后进行 Git commit。
8.  不提前引入没有实际需求的中间件。
9.  不把示例输出、模拟节点数据当成真实运行结果。
10. 对未验证的多机能力明确标记为 PoC，不写成已实现功能。

建议 Git 分支：

-   `main`：稳定版本；
-   `develop`：日常集成；
-   `feature/agent-gpu-monitor`；
-   `feature/coordinator-node-api`；
-   `feature/frontend-dashboard`；
-   `feature/vllm-poc`。

提交信息示例：

``` text
feat(agent): add NVIDIA GPU information collector
feat(server): add node registration API
feat(server): add heartbeat endpoint
feat(frontend): add node dashboard
test(agent): add GPU parser tests
docs: update MVP development plan
```

------------------------------------------------------------------------

## 12. 当前项目状态与下一步

### 12.1 已完成

-   [x] 明确项目定位：朋友之间私有 GPU 算力共享平台。
-   [x] 明确控制平面中心化、计算平面分布式。
-   [x] 明确当前以成都朋友节点为第一批接入对象。
-   [x] 确定优先研究 vLLM。
-   [x] Windows 11 开发环境准备。
-   [x] Go 1.24.0。
-   [x] Node.js 24.11.1 / npm 11.6.2。
-   [x] Git 2.56.0。
-   [x] Docker Desktop 4.97.0 / Engine 29.9.2 / Compose 5.6.1。
-   [x] WSL2 和 Ubuntu 启动验证。
-   [x] RTX 3050 4GB 的 `nvidia-smi` 验证。

### 12.2 尚未完成

-   [ ] 创建项目目录和 Git 仓库。
-   [ ] 初始化 Server Go Module。
-   [ ] 初始化 Agent Go Module。
-   [ ] 初始化 Vue 前端。
-   [ ] Docker Compose 启动 PostgreSQL。
-   [ ] Worker GPU Monitor。
-   [ ] Coordinator 健康检查和数据库连接。
-   [ ] 节点注册与心跳。
-   [ ] 节点和集群管理。
-   [ ] Web 控制台。
-   [ ] 多节点接入。
-   [ ] vLLM 单机 PoC。
-   [ ] vLLM 多机 PoC。

### 12.3 下一项实际开发任务

**从 Phase 0 的项目仓库初始化开始，然后立即执行 Phase 1：Worker GPU
采集最小程序。**

建议首个代码交付：

``` text
agent/
├── cmd/
│   └── agent/
│       └── main.go
├── internal/
│   └── gpu/
│       ├── nvidia.go
│       ├── types.go
│       └── nvidia_test.go
└── go.mod
```

首个程序只做一件事：

``` text
运行 Agent
   |
   v
调用 nvidia-smi
   |
   v
解析 GPU 信息
   |
   v
输出结构化 JSON
```

首个验收标准：在当前 Windows 11 电脑上，成功输出 RTX 3050 的 GPU
UUID、型号、总显存、已用显存、空闲显存、利用率和温度。

完成后再开发 Coordinator，不提前引入数据库、前端或 vLLM。

------------------------------------------------------------------------

## 13. 关键技术参考

-   vLLM GPU
    安装文档：<https://docs.vllm.ai/en/stable/getting_started/installation/gpu/>
-   vLLM
    分布式推理文档：<https://docs.vllm.ai/en/stable/serving/parallelism_scaling/>
-   Docker Desktop：<https://www.docker.com/products/docker-desktop/>
-   Docker Desktop WSL2：<https://docs.docker.com/desktop/features/wsl/>
-   Go：<https://go.dev/doc/>
-   Gin：<https://gin-gonic.com/docs/>
-   PostgreSQL：<https://www.postgresql.org/docs/>
-   Vue：<https://vuejs.org/guide/>
-   NVIDIA Management
    Library：<https://developer.nvidia.com/management-library-nvml>

> 技术版本、GPU 支持范围、驱动要求和 vLLM 多机功能会持续变化。真正进入
> vLLM PoC
> 时，应以当时官方文档和实际硬件测试结果为准，不在项目总纲中永久固定未经验证的镜像
> tag 或 CUDA 组合。
