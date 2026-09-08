# 功能落地与验收

每个模块按流程、设计、实现、测试推进。真实调用共用持久预算，本轮验收终点是本地 RTMP。

| 模块 | 流程与设计 | 实现落点 | 工程化测试 |
|---|---|---|---|
| 人物 | 导入首尾帧 → 内容摘要文件 → 发布不可变版本 → 会话固定引用 | character、store、session | 版本切换不改历史会话、损坏文件拒绝、数据库恢复 |
| 预算 | 预估 → 事务预占 → 调用 → 结算；未知结果保留预占 | generation、store、inference | 并发不超额、重复提交拒绝、重启不重置 |
| 对账 | 持续查询已知任务；未知提交人工绑定任务 ID | session、server | 无任务 ID 不重发、取消失败继续查询、绑定校验 |
| 连续性 | 前段实际尾帧 → 后段首帧；重规划使依赖后续失效 | session、ffmpeg | 缺前段等待、父素材变更拒绝、媒体抽帧 |
| 视觉 | 基准图与生成帧 → M3 判断 → 记录结果 → 通过才 READY | generation、inference、store | 拒绝不能播放、错误仍记费、播放后才更新事实 |
| 记忆 | 播放确认 → 视觉事实与游标同事务保存 → 下次开播恢复 | session、store | 回滚不写记忆、跨会话读取 |
| 互动 | 聚合弹幕 → M3 观察与导演 → 可修改未来 → 媒体输出 | audience、director、session | 冻结边界、重排、模拟弹幕端到端 |
| 音频 | 短台词与固定音色 → 合成 → 时长检查 → 媒体合并 | inference、ffmpeg、session | 合成失败、超长拒绝、媒体格式；口型人工验收 |
| 本地推流 | 真实 RTMP 服务 → 连续发送 → 接收端解码 | streaming、集成验收 | 本地接收媒体持续时间、音视频轨、退出回收 |

注意：代码测试、真实供应商测试、内容人工验收分别记录；未经执行不得视为通过。

## 本轮结果（2026-09-07）

- 人物：已将用户认可的虚化首尾帧登记为 `host` 的固定版本；数据在开发库和 `data/media`。数据库集成测试覆盖旧版本恢复、会话固定版本、复用原图修改、文件损坏拒绝。
- 预算与对账：并发预占、重启不重置、重复请求拒绝、绑定失败回滚、取消后继续查询测试通过。配置页展示全局已结算和预占费用。停止后仍对账已知任务，无编号需人工绑定。
- 连续性与视觉：真实 H3 Max 连续片段已生成，M3 因取景比例偏差拒绝；记录在 `data/acceptance/continuous-v1`。拒绝是内容验收未通过，不算生成质量达标。首/中/尾帧测试采用逐帧变化视频确认提取准确。
- 记忆：视觉事实与播放事务一起提交；未播出、提交失败、重复回执测试通过。只保存可见事实，不把提示词当作事件。
- 互动：真实 M3 弹幕观察和导演调用通过，产物在 `data/acceptance/audience`；仍需更多场景评估指令遵循程度。
- 音频：32kHz 固定系统音色合成后合入 48kHz AAC 音轨通过，产物在 `data/acceptance/speech`；超过片段时长拒绝的测试通过。口型重定时尚未实现，音色尚未由用户定稿。
- 本地 RTMP：使用项目 FFmpeg 输出模块推送已生成视频，MediaMTX 接收后再拉流，录得 600.12 秒 H.264/AAC，产物在 `data/acceptance/rtmp-long`。这验证传输持续性，不验证实时生成吞吐。
- 工程化：完整 `make go-verify` 通过；新增媒体与角色测试后集成验收再次通过。`make web-verify` 通过，包含 12 个单元测试和 8 个浏览器测试。未提交、未推送。

真实生成吞吐、长时间人物稳定性、口型与语音一致性仍不应宣称达标。本轮按用户要求只验收本地 RTMP，不进行 Bilibili 推流。

## 开播旅程与前端职责（2026-09-08）

执行顺序：统一开播条件与阶段 → 就地配置和状态反馈 → 拆分表单与数据层 → 针对失败分支和刷新恢复验收。

- `session/readiness` 定义开播规则；入口读取人物素材、凭据和账本。GET 展示检查结果，Start 再次检查。按启动缓冲视频费用下限判断额度，推理和语音仍由现有原子预占约束；不把下限描述为总价或可直播时长。
- `lib/journey` 只做状态推导；`lib/readiness-api` 负责 HTTP 与 Zod 校验；`queries/journey` 管理请求、命令和失效更新。页面不复制服务端快照，SSE 与 HTTP 继续使用单调 revision。
- `LivePreparation` 承载本场摘要与主操作；`LiveSessionControls` 管理停止及备用画面交互；`RuntimeDetails` 按需展示诊断。阻塞和运行异常保持可见。
- `ConfigPage` 只组装人物、生成、弹幕和备用服务。独立表单保存草稿，远端状态由对应 Query hook 提供。组件不得直接依赖 API 模块，由 dependency-cruiser 检查。
- 本地播放器改为同源 `/live-preview/`，由后端依据本地 RTMP 路径转发到 MediaMTX，避免局域网设备访问自己的 loopback。外部推流目标未配置本地播放器，不把参考图当作实时直播。

质量取舍：保持 500 行非空非注释文件上限、类型检查、React Hooks 规则、无循环依赖及未使用代码检查；不新增框架或通用业务基类。行数是上限，拆分依据为状态所有权与真实职责，不为满足数字制造碎片。重点验证重复启动、不足额度、保存失败保留草稿、准备期停止和页面刷新恢复。

依据：React《You Might Not Need an Effect》（https://react.dev/learn/you-might-not-need-an-effect）、TanStack Query Query Invalidation（https://tanstack.dev/query/latest/docs/framework/react/guides/query-invalidation）、Oxlint max-lines（https://oxc.rs/docs/guide/usage/linter/rules/eslint/max-lines）。

本次验收：`make go-verify` 通过（含真实 PostgreSQL/FFmpeg 集成测试）；追加入口测试后 `go test -race ./cmd/live` 与 `make lint` 通过。最终 `make web-verify` 通过，包含 13 个单元测试和 12 个浏览器测试。浏览器实查待机页面、额度阻塞提示和原页配置展开/返回。未调用付费生成，未重新执行跨设备实时播放验收；尚未提交本次修改。
