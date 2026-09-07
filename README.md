# Streaming Slop

面向持久 AI 角色的缓冲式生成直播运行时。

Agent 规划未来，视频模型生成片段，Go 管理时间轴与缓冲，FFmpeg 输出连续直播流。

```text
World → Director → Timeline → Generator → Stream
```

## 功能

- **连续直播**：异步生成、缓冲调度、片段提交与 FFmpeg 推流。
- **AI 编排**：Qwen 负责观察与导演，MiniMax 负责视频生成。
- **观众互动**：接入 Bilibili 弹幕，影响尚未提交的未来片段。
- **运行控制台**：查看时间轴、生成任务、成本与会话历史，调整模型配置。
- **状态与监控**：PostgreSQL 持久化、会话恢复和 Prometheus 指标。

采用 Go 模块化单体，内嵌 React + TypeScript 控制台。持续播放的前提是长期平均生成吞吐量高于播放速度，缓冲用于吸收延迟与抖动。

## 快速开始

准备 Go 1.26.6、Node.js 24、FFmpeg（含 `ffprobe`）及 Docker Compose，并获取 Qwen 与 MiniMax API Key。

```sh
git clone https://github.com/MariexAos/streaming-slop.git
cd streaming-slop

# 启动 PostgreSQL 和本地 RTMP 服务
docker compose up -d --wait

# 配置运行环境
export DATABASE_URL='postgres://streaming_agent:streaming_agent@127.0.0.1:5432/streaming_agent?sslmode=disable'
export RTMP_URL='rtmp://127.0.0.1:1935/live'
export STORY_SEED='一位 AI 主播与观众分享日常，并根据弹幕展开话题。'
export DASHSCOPE_API_KEY='your-qwen-api-key'
export MINIMAX_API_KEY='your-minimax-api-key'

# 安装前端依赖、构建并启动
make web-install
make run
```

打开 [本地控制台](http://127.0.0.1:8080) 配置并启动会话。数据库表会在服务启动时自动迁移；推流后可通过 [本地播放器](http://127.0.0.1:8888/live) 观看。

## 配置

| 环境变量 | 用途 | 默认值 |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL 连接地址 | 必填 |
| `RTMP_URL` | 推流目标地址 | 必填 |
| `STORY_SEED` | 故事或直播主题 | 必填 |
| `DASHSCOPE_API_KEY` | Qwen API Key | 可在控制台配置 |
| `MINIMAX_API_KEY` | MiniMax API Key | 可在控制台配置 |
| `CHARACTER_DESCRIPTION` | 角色描述 | 内置主播设定 |
| `BILIBILI_ROOM_ID` | 弹幕房间号 | `0`，不连接 |
| `BILIBILI_COOKIE` | Bilibili 连接凭据 | 空 |
| `MINIMAX_MAX_CONCURRENCY` | 视频生成并发上限 | `4` |
| `DATA_DIR` | 本地媒体目录 | `./data` |

完整环境配置见 [config.go](internal/config/config.go)。控制台默认监听 `127.0.0.1:8080`。

## 开发

```sh
make test       # Go 测试
make web-test   # 前端测试
make build      # 构建前端与 Go 可执行文件
```

代码按业务能力组织在 `internal/`；生成供应商位于 `generation/`，媒体实现位于 `streaming/`，共享事务位于 `store/`，HTTP 与控制台位于 `server/`。具体实现依赖用例包，核心 `live` 只依赖 Go 标准库。

更多说明见 [设计文档](docs/README.md)、[架构与代码边界](docs/01-architecture.md) 和 [渐进交付计划](docs/06-delivery-plan.md)。

## 前端质量验收

需要 Go（版本见 `go.mod`）、Node.js 24 和 pnpm 11.19.0。仓库根目录执行：

```sh
make web-verify
```

该命令按顺序执行冻结锁文件安装、Chromium 及系统依赖安装、Vite+ 格式/类型/React 检查、dependency-cruiser 架构检查、Knip、单元测试、生产构建、Playwright 浏览器测试、Go 嵌入资源测试和可执行文件构建。Linux 首次安装浏览器系统依赖可能需要 sudo；无网络或进程/端口权限应报告环境失败，不能跳过后声称验收通过。

开发迭代从失败阶段继续：

| 阶段 | 在 `web/` 中执行 |
|---|---|
| 自动格式化和安全修复 | `pnpm exec vp check --fix` |
| 格式、类型和 React 检查 | `pnpm exec vp check` |
| 面向 agent 的 lint 诊断 | `pnpm exec vp lint --format agent --deny-warnings` |
| 架构依赖 | `pnpm run lint:arch` |
| 未使用文件、导出和依赖 | `pnpm exec knip` |
| 单元测试 | `pnpm test` |
| 构建 | `pnpm run build` |
| 浏览器测试（需已有最新构建） | `pnpm run test:e2e` |
| 静态检查、单测和构建组合 | `pnpm run quality` |

所有前端源文件统一遵守 500 个非空、非注释行上限，不设历史基线。不得通过 disable、忽略源码、放宽阈值或跳过测试解决失败。Vitest/Playwright 禁止独占测试，浏览器测试不通过重试掩盖失败。生成产物和 pnpm 锁文件由工具维护，不属于手工源码格式化范围。

`frontend-quality`、`frontend-browser` 在 CI 使用同一套命令；`backend-quality` 复用前端产物并额外运行 Go 全量门禁。`make web-verify` 验证前端及 Go 嵌入边界，不替代后端的全量检查。分支保护中的 required checks 属于 GitHub 仓库设置，需要启用后才能阻止合并。

当前取舍：Vite+ 统一工具入口；pnpm 锁定依赖；React 基础规则使用原生实现，其余推荐规则直接加载官方 Hooks 插件；不引入无障碍检查。`pnpm-workspace.yaml` 中 Vite 版本映射用于识别 Vite+ core 的包版本（0.3.0）与其内置 Vite 8 的兼容关系，不关闭代码质量规则。

编码代理的具体执行流程见 [web/AGENTS.md](web/AGENTS.md)。

## 后端质量验收

Go 执行版本来自 `go.mod` 的 `toolchain`；Makefile 固定 golangci-lint 和 govulncheck 版本。首次运行自动安装到 `bin/`，lint 使用官方二进制并核对发布校验和。需要 curl、tar、shasum，以及集成阶段使用的 FFmpeg/ffprobe 和 PostgreSQL 16。

```sh
# 启动独立、临时的测试数据库，不使用开发数据库
# 数据保存在 tmpfs，停止容器后不保留。
docker compose -p streaming-quality -f compose.quality.yaml up -d --wait
export TEST_DATABASE_URL='postgres://quality:quality@127.0.0.1:55432/streaming_quality?sslmode=disable'
make go-verify
docker compose -p streaming-quality -f compose.quality.yaml down
```

测试会清理数据，`TEST_DATABASE_URL` 必须指向可丢弃的测试数据库。

| 入口 | 验收内容 |
|---|---|
| `make format` | 自动格式化 Go 源码 |
| `make go-quality` | 配置与模块校验、格式、文件大小、全量 lint、race 与覆盖率、构建、漏洞扫描 |
| `make go-integration` | PostgreSQL 迁移/恢复/原子更新、旧结果拒绝、FFmpeg 规格、fallback 与本地播放连续性 |
| `make go-verify` | 按顺序执行上述两阶段；失败后可从对应阶段继续 |
| `make go-soak` | 十分钟真实本地媒体运行；同样需要测试数据库与媒体工具 |
| `make go-fuzz` | 30 秒冻结边界与重规划 fuzz；可用 `FUZZ_TIME=2m` 延长 |
| `make check` | 前端完整验收，再执行后端完整验收 |

普通 Go 测试不包含带 `integration`/`soak` 标签的环境测试。集成入口禁用测试缓存、串行运行包，并核对 JSON 报告：必需的数据库和媒体测试必须全部通过，skip、缺失和失败都不能算成功。静态分析同时检查这两种测试标签。测试不调用付费模型 API，使用可控生成器及真实数据库和 FFmpeg。

Go 源码统一限制为 500 个非空、非注释行，无文件特例；生成代码除外。生产函数限制为 80 行、50 条语句，认知复杂度报告阈值为 20；测试函数免除这两项结构指标，但仍受文件大小、正确性和适用的架构规则约束。`live` 只依赖标准库；生产用例不得导入 server、store、config、具体供应商实现或 cmd；外层包不得导入 cmd。跨层集成测试按其测试职责处理。

覆盖率报告位于 `.artifacts/coverage.{out,txt,html}`，表示普通测试覆盖率，暂不设置全仓百分比阈值。集成执行证据保存在 `.artifacts/integration.json`，持续播放报告保存在 `.artifacts/soak.txt`。媒体验收检查音视频 DTS 严格递增、相邻包间隔不超过 250ms，并检查实际媒体时长。

CI 的 `backend-quality` 和 `backend-integration` 复用相同 Make 入口。`backend-extended` 每周及手动选择 extended 时执行十分钟持续播放与两分钟 fuzz；不加入每次 PR 的快速门禁。Actions 固定完整提交 SHA，Dependabot 每周检查 Go 依赖及 Actions 更新。Go 工具版本升级应修改 Makefile，并重新完成验收。

需要在 GitHub 将 `frontend-quality`、`frontend-browser`、`backend-quality`、`backend-integration` 设置为必过检查，才能阻止不合格代码合并。当前私有仓库的 Ruleset/分支保护 API 返回套餐限制 403，需要支持此功能的套餐才能启用；本地验收不能替代远端 CI 运行记录。

## 生成供应商与验收边界

默认保留 MiniMax 直连。使用 fal Turbo 时设置 `GENERATION_PROVIDER=fal`、`FAL_KEY`；默认端点为 `minimax/h3-max-turbo/image-to-video`，可用 `FAL_MODEL` 和 `FAL_BASE_URL` 指定已支持的端点与地址。停止会话后重启切换供应商。fal 配置在控制台只读，人民币成本未知时显示“—”，不能当作免费。

`SEGMENT_DURATION_SECONDS` 接受 5–15 的整数；默认 5。媒体准备按请求时长处理，业务片段与 FFmpeg 传输分片独立。控制台保存 API Key 时保留当前时长。

请求在提交前保存模型、参数、参考素材版本与片段计划版本。提交响应丢失时保留待核对记录，不自动再次付费提交。生成结果经过版本检查，过期结果不能成为当前素材。

开发数据库使用 `compose.yaml`，`compose.quality.yaml` 只用于可丢弃测试。`make go-soak` 使用模拟生成器和真实数据库/FFmpeg；它验证媒体链路，不验证模型人物一致性。

### 人物与预算

首尾帧首次启动时导入 `data/media`，元信息和不可变版本保存到 PostgreSQL。配置页可以上传新版本、复用原图修改描述或音色、选择下一场直播的版本；进行中的会话继续使用原版本。恢复数据需要同时保留数据库和媒体目录。

运行时默认使用 MiniMax M3 观察弹幕、导演和视觉检查，复用 MiniMax 密钥。选填人物音色 ID 后使用 Speech 2.8 Turbo 合成短台词，超过片段时长会拒绝；该步骤不提供口型重定时，默认留空保留视频原声。

本轮预算为 ¥10，持久预占和结算，重启不重置额度。停止后后台继续对账已知任务；提交结果不明时在配置页绑定供应商任务编号，不能清空账本后重新提交。

各模块流程、设计和测试见 [功能落地与验收](docs/07-implementation.md)。真实验收入口：

```sh
go run ./tools/acceptance -mode setup
go run ./tools/acceptance -mode audience -output data/acceptance/audience
go run ./tools/acceptance -mode inspect -source data/acceptance/blurred-frames/video.mp4
go run ./tools/acceptance -mode generate -source data/acceptance/blurred-frames/video.mp4 -output data/acceptance/next
go run ./tools/acceptance -mode speech -output data/acceptance/speech
go run ./tools/acceptance -mode rtmp -source data/acceptance/blurred-frames/video.mp4 -duration 10m -output data/acceptance/rtmp-long
```

除 RTMP 外需配置 `DATABASE_URL`；密钥读取开发库。`generate` 只生成一个片段，有提交记录的目录不能再次提交。`inspect`、`audience`、`speech` 也消耗预算。RTMP 使用既有视频循环验证本机 1935 端口的推拉流，不代表模型能实时持续供片。
