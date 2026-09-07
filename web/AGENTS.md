# 前端编码与质量反馈

- 保持 Vite+ + pnpm 工具链；版本以 package.json 和 pnpm-lock.yaml 为准。不得新增独立 ESLint、Prettier 或无障碍门禁。
- 完整本地验收入口是仓库根目录的 `make web-verify`，具体阶段见根 README 的“前端质量验收”。提交本次最终结果前，该流程必须在最终代码上通过；已经通过且输入未变化的阶段可以复用结果。
- 每次修改先执行最直接相关的检查。真实失败后，依据工具报告的文件、规则、错误分支进行最小修复，再从失败阶段继续。`pnpm exec vp lint --format agent --deny-warnings` 提供 agent 可读诊断。
- 不允许文件级阈值例外、eslint/oxlint disable、ts-ignore/ts-nocheck、忽略业务源码、测试 skip/todo/only、通过重试或调低规则强度获得绿色结果。不得用类型断言掩盖未验证的接口数据。
- 文件上限统一为 500 个非空、非注释行。超限时按真实职责拆分，不压缩格式或创建没有业务意义的碎片。
- App 只负责页面组装和视图选择；queries 管理服务端缓存、请求、mutation 与 SSE；store 只保留连接状态与客户端指标历史；components/ui 不依赖业务组件、store 或 API；lib 不依赖 queries、store 或 UI；实际 import 边界以 dependency-cruiser 配置为准。
- Effect 显式声明依赖。Zustand 必须按字段订阅；页面就近订阅 Query，不将完整快照放入 App 或复制到 Zustand；不能将变化的整个快照作为重连触发条件；连接、定时器和请求必须清理。
- 接口数据在 API 边界经过 Zod；测试 fixture 与测试用例分离，生产源码不能依赖测试或 fixture。
- 环境权限、下载失败或缺少依赖不是检查通过。记录具体失败命令和原因，解决环境问题后继续，不绕过门禁。
- 最终报告列出实际执行的命令、测试结果和未完成项；不得把本地成功描述成远端 CI 已通过。确认修改范围后再提交，未经请求不推送。
- GET 请求传递 Query 的 AbortSignal；写操作不自动重试，以服务器响应更新缓存。HTTP 与 SSE 快照共用单调 revision 规则。表单草稿留在组件，不用 Effect 将查询结果复制到本地 state。
