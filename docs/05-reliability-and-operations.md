# 可靠性与运行

[上一篇：端口与 Adapter](04-ports-and-adapters.md) · [返回索引](README.md) · 下一篇：[渐进交付](06-delivery-plan.md)

## 最小失败模型

第一阶段只明确处理会中断直播的四类失败：

| 失败 | 处理 |
|---|---|
| Submit 超时、结果未知 | 使用幂等键查询或重试，不创建重复有效 Attempt |
| webhook 丢失 | 低频 reconciliation 查询未决 Job |
| 生成失败或资产无效 | 有上限地重试；临近播放时转 fallback |
| FFmpeg/RTMP 中断 | 保留 playhead 与已提交序列，重启输出并记录 gap |

重试策略由 generation 用例统一决定，默认次数保持很小并带退避；鉴权、参数错误等永久失败不重试。不要写无限循环或“捕获一切后继续”。

## Fallback

从第一天准备一组已标准化、可循环的安全片段，例如 idle、看屏幕、喝水、思考和 standby。

```text
ReadyAhead < 5 秒 -> 选择与当前场景最接近的 fallback
                   -> 在下一个 Segment 边界接入
Provider 恢复      -> 先重新建立 Ready Buffer
                   -> 再在边界回到生成内容
```

Fallback 是正常运行模式之一，不是 panic 后的临时补丁。它的播放时长和触发原因必须被记录。

## Safety

```text
Audience -> Direction -> Safety Gate -> Generation
Generated Asset -> 可选输出审核 -> Commit
```

- Director 不是 Safety Gate。
- 默认保留供应商安全检查。
- 管理员可停止会话或切换到预制安全画面。
- 输出审核如果无法满足实时预算，先限制生成范围并保留人工切换，不阻塞整个 MVP。

## 可观测链路

每个 Segment 使用同一个关联标识串起：

```text
Direction -> Prompt -> Provider request -> Attempt timing
          -> Asset -> Normalize -> Commit -> Playback
```

第一屏只需要这些指标：

| 指标 | 用途 |
|---|---|
| `live_ready_seconds` | 是否即将断流 |
| `live_submitted_seconds` | 未来任务覆盖范围 |
| `generation_latency_seconds` | P50/P95 与并发计算 |
| `generation_failure_total` | 供应商健康和重试成本 |
| `generation_cost_usd` | 每分钟成本与浪费 |
| `fallback_seconds_total` | 服务质量退化 |
| `stream_dropped_frames_total` | 媒体输出健康 |
| `stream_bitrate` | 推流质量 |

日志使用结构化字段：`session_id`、`segment_id`、`attempt_id`、`provider_job_id`。Prompt 和观众原文按数据策略采样或脱敏，不默认全部写日志。

成本同时记录每次 Attempt 和最终是否播放，以计算：

```text
$/generated minute
$/live minute
waste ratio = 未播放生成成本 / 总生成成本
retry cost ratio
```

## 配置

配置按语义分组：timeline、scheduler、director、generator、media、stream、storage、observability。密钥只从环境或密钥文件注入，不进入配置文件或日志。

启动时只校验当前运行模式必需的字段。开发模式不因缺少 Bilibili 或生产对象存储配置而拒绝启动。

## 部署演进

MVP 在一台普通 CPU 主机运行：

```text
Go Runtime + PostgreSQL + local disk + FFmpeg [+ OBS]
                         |
                         +---- HTTPS ---- video provider
```

需要自建模型后，再通过相同 `VideoGenerator` 端口连接 GPU worker。只有在独立扩缩容、故障隔离或团队所有权成为真实需求时，才拆 generation/media/platform 服务并评估 NATS 或 Kafka。
