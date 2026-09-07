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

完整环境配置见 [config.go](internal/platform/config/config.go)。控制台默认监听 `127.0.0.1:8080`。

## 开发

```sh
make test       # Go 测试
make web-test   # 前端测试
make build      # 构建前端与 Go 可执行文件
```

代码按业务能力组织在 `internal/`，外部集成位于 `internal/adapter/`，前端源码位于 `web/`。依赖方向为 `adapter/platform → 用例包 → live`，核心 `live` 包只依赖 Go 标准库。

更多说明见 [设计文档](docs/README.md)、[架构与代码边界](docs/01-architecture.md) 和 [渐进交付计划](docs/06-delivery-plan.md)。
