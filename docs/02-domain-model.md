# 核心模型与不变量

[上一篇：架构](01-architecture.md) · [返回索引](README.md) · 下一篇：[运行时流程](03-runtime.md)

## 状态边界

`LiveSession` 是一次直播的顶层状态，持有当前 `WorldState`、`Timeline` 和 `StreamState`。长耗时的生成 Job 不在一个数据库事务里执行，只通过标识和状态与 Segment 关联。

```go
type LiveSession struct {
	ID       SessionID
	Status   SessionStatus
	World    WorldState
	Timeline Timeline
	Stream   StreamState
	CreatedAt time.Time
}
```

字段示例表达语义，不要求一开始逐字实现；优先让核心方法维护规则。

## Timeline

Timeline 是按直播会话相对时间排列的 Segment 序列。所有时间边界使用 `time.Duration`，墙上时间只用于事件审计。

关键量：

- `Playhead`：当前正在播放的位置，只能前进。
- `CommitHorizon`：`Playhead` 之后即将冻结的范围。
- `ReadyAhead`：从 Playhead 开始连续可播放的时长；不能把中间有洞的 READY 时长相加。
- `SubmittedAhead`：已有资产或有效生成任务覆盖到的最远连续位置。

```text
PLAYED | PLAYING | COMMITTED | READY | GENERATING | PLANNED
                 ^           ^
              不可修改      可重规划
```

### Timeline 不变量

1. Segment 时长必须为正，相邻 Segment 不重叠；MVP 中必须连续。
2. 一个时间位置最多对应一个生效 Segment。
3. Playhead 单调递增。
4. Commit Horizon 内的 Direction 和 Asset 不可替换。
5. 播放只能按时间顺序发生，不能跳过未处理的洞。
6. ReadyAhead 必须由连续、已验证可播放的资产计算。

## Segment

Segment 表示“某段时间最终要播什么”，而不是某次供应商请求。

```go
type Segment struct {
	ID         SegmentID
	Start      time.Duration
	End        time.Duration
	Direction  Direction
	Generation GenerationState
	Asset      *VideoAsset
	Status     SegmentStatus
}
```

主状态机：

```text
PLANNED -> GENERATING -> READY -> COMMITTED -> PLAYING -> PLAYED
    |           |
    +-----------+----> replanned（仅在 commit 前）
```

`READY != COMMITTED`：READY 资产仍可因观众互动被丢弃并重规划；COMMITTED 表示它已经进入播放安全区。

供应商 Job 的失败、取消和重试属于 `GenerationAttempt`，不要把所有 Job 状态塞进 Segment 状态机：

```text
SUBMITTED -> RUNNING -> SUCCEEDED
                  \-> FAILED
SUBMITTED/RUNNING  -> CANCELLED
```

一次 Attempt 失败后，generation 用例决定创建下一次 Attempt，或为 Segment 选择 fallback。

## WorldState

WorldState 回答“这个世界此刻是什么样”，只保存生成和连续性真正需要的事实。

```go
type WorldState struct {
	Character CharacterState
	Scene     SceneState
	Camera    CameraState
	Story     StoryState
	Recent    []WorldEvent
}
```

- 当前状态是结构化事实，不是完整聊天记录。
- 每个已播放 Segment 可产生一个 `WorldDelta`，按顺序应用到 WorldState。
- 必要节点保存 snapshot，便于恢复；不追求通用 event sourcing。
- 角色身份、服装、场景和光照引用放在 `AnchorSet`，不只依赖上一段末帧。

## Direction 与计划层级

Director 只输出模型无关的 Direction：

```go
type Direction struct {
	Action     string
	Dialogue   string
	Emotion    string
	Camera     CameraDirection
	Continuity ContinuityConstraint
}
```

计划分三层，越近越具体：

| 时间范围 | 产物 | 是否生成视频 |
|---|---|---|
| 10～20 分钟 | Story Plan / beats | 否 |
| 2～3 分钟 | Direction 草案 | 否 |
| 60～90 秒 | Generation Attempt | 是 |
| 45～60 秒 | Ready Asset | 已完成 |
| Commit Horizon 内 | Committed Segment | 不可改变 |

Prompt 是供应商方言，由 Prompt Compiler 从 `WorldState + Direction + AnchorSet` 生成，不进入核心领域。

## 最小持久化语义

需要恢复的事实包括：

- session、当前 world snapshot 和 playhead；
- segment、direction、status 和有效 asset；
- generation attempt、provider request ID、状态、耗时和成本；
- audience event 聚合结果与 story plan；
- asset 的位置、媒体属性和校验结果。

原始 MP4、参考图像、音频和录像存对象存储或本地磁盘；数据库只保存元数据和引用。
