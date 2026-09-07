# 端口与 Adapter

[上一篇：运行时流程](03-runtime.md) · [返回索引](README.md) · 下一篇：[可靠性与运行](05-reliability-and-operations.md)

## 端口原则

端口由用例使用方就近定义，用核心语言表达，不泄漏供应商 DTO。只有跨进程、跨协议、跨持久化边界，或确实需要替换的能力才抽象。不创建统一的 `ports` 层。

示意接口：

```go
type Director interface {
	Direct(context.Context, DirectionInput) (live.Direction, error)
}

type PromptCompiler interface {
	Compile(context.Context, PromptInput) (GenerationSpec, error)
}

type VideoGenerator interface {
	Submit(context.Context, GenerationSpec) (GenerationJob, error)
	Status(context.Context, ProviderJobID) (JobStatus, error)
	Result(context.Context, ProviderJobID) (GeneratedAsset, error)
}

type StreamOutput interface {
	Start(context.Context) error
	Write(context.Context, MediaSegment) error
	Stop(context.Context) error
}

type SessionRepository interface {
	Load(context.Context, live.SessionID) (*live.LiveSession, error)
	Save(context.Context, *live.LiveSession) error
}
```

真实接口应由第一个用例推导后再落地；不要一次性创建“未来可能需要”的方法。

## Adapter 清单

| 方向 | Adapter | 责任 |
|---|---|---|
| 入站 | HTTP / CLI | DTO 与 Command 转换、返回状态 |
| 入站 | Bilibili | 平台事件归一化、重连和 cursor |
| 出站 | LLM Director | 结构化输出、模型错误映射 |
| 出站 | Prompt Compiler | Direction 到模型方言的确定性编译 |
| 出站 | fal generator | 提交、状态、结果和 webhook 验证 |
| 出站 | FFmpeg | normalize、concat/mux、持续输出 |
| 出站 | Postgres | 聚合和 Attempt 元数据持久化 |
| 出站 | local/S3 asset | 大文件存取和定位 |

Provider routing、重试时机和 fallback 选择属于 generation 用例；Adapter 只报告能力、限制和规范化错误。

## 供应商隔离

每个外部集成至少有三种类型：

```text
provider request/response  <->  mapper  <->  用例包公开类型
```

- Provider request ID 可以保存，但不能成为 Segment ID。
- 供应商状态映射到少量内部 JobStatus；未知状态返回明确错误，不扩散字符串判断。
- Prompt Compiler 与生成 Adapter 同属供应商模块，但保持独立类型，方便基准测试。
- Reference 数量、时长、分辨率和 rate limit 是 Adapter capability，不写成核心常量。
- 通过 Segment ID 和 Attempt number 生成稳定幂等键，处理提交超时后的不确定结果。

## 存储边界

MVP 使用 PostgreSQL 加本地磁盘即可：

```text
PostgreSQL: session / segment / attempt / event summary / plan / metadata
Local disk: generated media / anchors / normalized media / recordings
```

替换为对象存储时，只更换 AssetStore Adapter。不要让 `live` 拼接文件路径或对象 URL。

一次状态推进采用短事务：读取当前版本、验证转换、保存新状态和 outbox-like 待办记录。第一阶段不实现通用 Outbox 框架；只有实际出现跨进程投递需求时再引入。

## 测试策略

- `live`：表驱动测试覆盖状态转换、边界和不变量。
- 业务能力包：使用手写 fake 验证用例顺序、优先级和失败分支。
- `adapter`：针对真实协议的契约测试；供应商测试由环境变量显式开启。
- `media`：使用短固定样本验证时长、时间戳、编码和连续播放。
- 端到端：先跑本地 10 分钟，再接 RTMP；不在普通单元测试中启动外部模型。

优先手写小 fake，不引入 mock 生成框架。
