# 渐进交付

[上一篇：可靠性与运行](05-reliability-and-operations.md) · [返回索引](README.md)

每一阶段必须产生一个可运行、可测量的纵向切片。没有通过当前阶段验收，不提前建设下一阶段基础设施。

## Phase 0：连续性实验

目标：验证同一角色可连续生成 `10 × 5s` 片段并合成为 50 秒视频。

实现范围：Go CLI、一个 Prompt Compiler、一个 VideoGenerator Adapter、媒体 normalize/concat、延迟与成本记录。

验收：人工检查人物/场景/音频连续性；输出每段生成耗时、失败和成本；同一输入可以追踪到产物。

## Phase 1：本地缓冲运行时

目标：连续本地播放 10 分钟。

实现范围：`live` 核心模型、timeline/generation 用例、Scheduler、异步 Attempt、Ready Buffer、fallback、FFmpeg 本地输出。先用 fake generator 验证时间轴，再接真实供应商。

验收：播放不断流；ReadyAhead 可观测；单次生成失败能恢复；重启后可对账未决 Job；无不可解释的时间戳跳变。

## Phase 2：真实推流

目标：先稳定推流 30 分钟，再稳定运行 2 小时。

实现范围：30 秒延迟 playhead、RTMP 输出、可选 OBS 场控、停止与恢复流程。

验收：平台侧无持续卡顿；fallback 可在边界切入和退出；记录 dropped frames、bitrate 和 gap。

## Phase 3：观众互动

目标：观众指令能改变可修改的未来，并在 30～60 秒后播出。

实现范围：Bilibili 入站 Adapter、5 秒事件聚合、Director、Direction 安全检查和未来重规划。

验收：已 COMMITTED Segment 从不被改写；高频弹幕不会逐条调用 LLM；互动结果可追踪到聚合事件和 Direction。

## Phase 4：持久角色

目标：角色在一次直播内保持目标、个性、场景和记忆，并开始支持跨直播延续。

实现范围：Strategist、Session Memory、少量长期记忆检索、AnchorSet、场景切换。

验收：2 小时运行中故事目标可解释；重启会话后能恢复关键事实；连续性指标和人工评分稳定。

## Phase 5：本地 GPU

目标：增加本地 VideoGenerator，而不改变 `live`、timeline、director 和 streaming。

验收：通过同一 Adapter 契约测试；可以按能力和健康状态路由；上层包不导入本地推理 SDK。

## 第一轮实现顺序

1. 建立 Go module 和 `internal/live`，用测试固定 Segment 与 commit 不变量。
2. 定义当前用例实际需要的 `VideoGenerator` 端口，并实现可控 fake。
3. 实现 Scheduler，用模拟延迟跑出稳定 ReadyAhead。
4. 接入真实 generator 和 Prompt Compiler，完成 Phase 0 基准。
5. 接入媒体 normalize 与本地输出，完成 Phase 1。
6. 通过验收后再增加数据库、RTMP 和互动入口。

## 现在不做

```text
微服务 / Kubernetes / Kafka / Redis
通用 DAG 或工作流引擎
多 Agent swarm
复杂向量知识图谱
LoRA 训练与自建 GPU 推理
多平台直播和 24/7 无人值守
Speculative generation
```

这些能力只有在现有架构出现可测量瓶颈时才进入新的设计决策。
