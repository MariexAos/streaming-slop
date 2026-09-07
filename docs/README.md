# Streaming Agent 设计文档

## 一句话

这是一个面向持久 AI 角色的缓冲式生成直播运行时：Agent 规划未来，视频模型生成未来片段，Go 管理时间轴与缓冲，FFmpeg 输出连续媒体流。

系统成立的基本条件是：

```text
长期平均生成吞吐量 > 播放速度
```

30～60 秒的可播放缓冲用于吸收生成延迟、抖动和有限失败。模型、直播平台和存储都只是可替换 Adapter。

## 先记住五个概念

```text
World -> Director -> Timeline -> Generator -> Stream
```

- **World**：角色和场景此刻是什么状态。
- **Director**：下一段应该发生什么。
- **Timeline**：何时发生，以及是否还允许修改。
- **Generator**：把 Direction 变成可播放片段。
- **Stream**：按顺序、无中断地播给观众。

Go 拥有执行权。LLM 只产出结构化的 `Direction`，不直接调用视频 API、控制重试或操作 FFmpeg。

## MVP 基线

| 参数 | 初始值 |
|---|---:|
| Segment 时长 | 5 秒 |
| 直播延迟 | 30 秒 |
| Commit Horizon | 30 秒 |
| Ready Target | 45～60 秒 |
| Submitted Horizon | 60～90 秒 |
| 细节计划 | 2～3 分钟 |
| 故事计划 | 10～20 分钟 |

这些是可观测后再调整的默认值，不是领域常量。

## 阅读路径

只需理解方案时，读到第 2 篇即可；实现某个部分时再向下展开。

1. [架构与代码边界](01-architecture.md)：为什么是模块化单体，代码如何依赖。
2. [核心模型与不变量](02-domain-model.md)：Timeline、Segment、World 的精确定义。
3. [运行时流程](03-runtime.md)：规划、生成、提交、播放和互动如何闭环。
4. [端口与 Adapter](04-ports-and-adapters.md)：外部能力的最小接口与落点。
5. [可靠性与运行](05-reliability-and-operations.md)：降级、恢复、可观测与部署。
6. [渐进交付](06-delivery-plan.md)：每一阶段做什么、如何验收、暂不做什么。

## 已决定的边界

- 第一阶段采用 **Go 模块化单体**，单进程、单仓库、清晰包边界。
- Clean Architecture 在这里约束依赖方向，不规定 `application/domain` 目录；Go 包按业务能力组织。
- 以 `Timeline Segment` 为中心，不以 Prompt、供应商 Job 或 MP4 文件为中心。
- `READY` 与 `COMMITTED` 严格分离，互动只能改变尚未提交的未来。
- 第一阶段不引入微服务、Kafka、Kubernetes、工作流引擎、Redis 或多 Agent swarm。
- OBS 只用于开发和人工场控，不是生产运行时的必要依赖。
