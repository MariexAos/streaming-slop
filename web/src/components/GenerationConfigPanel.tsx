import { useState } from "react"
import {
  BrainCircuit,
  CheckCircle2,
  CircleDollarSign,
  Eye,
  MessageCircleMore,
  RadioTower,
  Save,
  Send,
  ServerCog,
} from "lucide-react"
import type { BilibiliConfig, GenerationConfig, QwenConfig } from "@/lib/schema"
import { formatNumber } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function GenerationConfigPanel({
  config,
  saving,
  message,
  error,
  onSave,
  bilibiliConfig,
  bilibiliSaving,
  bilibiliMessage,
  bilibiliError,
  onSaveBilibili,
  mockSaving,
  mockMessage,
  mockError,
  onSendMock,
  qwenConfig,
  qwenSaving,
  qwenMessage,
  qwenError,
  onSaveQwen,
}: {
  config: GenerationConfig | null
  saving: boolean
  message: string | null
  error: string | null
  onSave: (
    value: Pick<GenerationConfig, "model" | "resolution" | "durationSeconds" | "ratio"> & {
      apiKey?: string
    },
  ) => Promise<boolean>
  bilibiliConfig: BilibiliConfig | null
  bilibiliSaving: boolean
  bilibiliMessage: string | null
  bilibiliError: string | null
  onSaveBilibili: (value: { roomId: number; cookie?: string }) => Promise<boolean>
  mockSaving: boolean
  mockMessage: string | null
  mockError: string | null
  onSendMock: (value: { username: string; text: string }) => Promise<boolean>
  qwenConfig: QwenConfig | null
  qwenSaving: boolean
  qwenMessage: string | null
  qwenError: string | null
  onSaveQwen: (value: { baseUrl: string; apiKey?: string }) => Promise<boolean>
}) {
  const [apiKey, setAPIKey] = useState("")
  const [roomDraft, setRoomId] = useState<string>()
  const roomId = roomDraft ?? (bilibiliConfig?.roomId ? String(bilibiliConfig.roomId) : "")
  const [cookie, setCookie] = useState("")
  const [mockUsername, setMockUsername] = useState("本地观众")
  const [mockText, setMockText] = useState("")
  const [baseDraft, setQwenBaseURL] = useState<string>()
  const qwenBaseURL = baseDraft ?? qwenConfig?.baseUrl ?? ""
  const [qwenAPIKey, setQwenAPIKey] = useState("")

  if (!config)
    return <p className="text-sm text-[var(--text-muted)]">{error ?? "正在加载生成配置…"}</p>

  const save = async () => {
    const value = apiKey.trim()
    const saved = await onSave({
      model: "MiniMax-H3-Max",
      resolution: "768P",
      durationSeconds: 5,
      ratio: "16:9",
      ...(value ? { apiKey: value } : {}),
    })
    if (saved) setAPIKey("")
  }

  const saveBilibili = async () => {
    const parsedRoomId = Number(roomId)
    if (!Number.isInteger(parsedRoomId) || parsedRoomId <= 0) return
    const value = cookie.trim()
    const saved = await onSaveBilibili({
      roomId: parsedRoomId,
      ...(value ? { cookie: value } : {}),
    })
    if (saved) setCookie("")
  }

  const sendMock = async () => {
    const text = mockText.trim()
    if (!text) return
    if (await onSendMock({ username: mockUsername.trim() || "本地观众", text })) setMockText("")
  }

  const saveQwen = async () => {
    const apiKey = qwenAPIKey.trim()
    const saved = await onSaveQwen({ baseUrl: qwenBaseURL.trim(), ...(apiKey ? { apiKey } : {}) })
    if (saved) setQwenAPIKey("")
  }

  const fieldClass =
    "mt-2 min-h-11 w-full rounded-xl border border-[var(--line-strong)] bg-[var(--surface-raised)] px-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
  return (
    <div className="space-y-5">
      <div>
        <p className="eyebrow">运行配置</p>
        <h2 className="section-title">MiniMax 视频生成</h2>
      </div>
      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>生成参数</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm font-semibold text-[var(--text-muted)] sm:col-span-2">
                MiniMax API Key
                <input
                  className={fieldClass}
                  type="password"
                  autoComplete="new-password"
                  value={apiKey}
                  onChange={(event) => setAPIKey(event.target.value)}
                  placeholder={
                    config.apiKeyConfigured ? "已配置；留空则保持不变" : "请输入 MiniMax API Key"
                  }
                />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                模型
                <input className={fieldClass} value="MiniMax-H3-Max（在线固定）" disabled />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                分辨率
                <input className={fieldClass} value="768P（在线固定）" disabled />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                画幅
                <input className={fieldClass} value="16:9（图生视频按 Anchor 自适应）" disabled />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                片段时长
                <input className={fieldClass} value="5 秒（直播契约固定）" disabled />
              </label>
            </div>
            <div aria-live="polite" className="min-h-5 text-sm">
              {error ? (
                <span className="text-rose-600 dark:text-rose-300">{error}</span>
              ) : message ? (
                <span className="text-emerald-600 dark:text-emerald-300">{message}</span>
              ) : null}
            </div>
            <Button variant="primary" disabled={saving} onClick={() => void save()}>
              <Save className="size-4" aria-hidden="true" />
              {saving ? "保存中…" : "保存配置"}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>连接与计费</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[var(--text-muted)]">供应商</span>
              <Badge tone="accent">
                <ServerCog className="size-3" />
                {config.provider}
              </Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-[var(--text-muted)]">API Key</span>
              <Badge tone={config.apiKeyConfigured ? "success" : "danger"}>
                <CheckCircle2 className="size-3" />
                {config.apiKeyConfigured ? "已配置" : "未配置"}
              </Badge>
            </div>
            <div className="rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-4">
              <div className="flex items-center gap-2 font-semibold text-[var(--text)]">
                <CircleDollarSign className="size-4 text-[var(--accent)]" aria-hidden="true" />
                当前成本（只读）
              </div>
              <dl className="mt-3 space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <dt className="text-[var(--text-muted)]">输出单价</dt>
                  <dd className="font-mono">
                    ¥{formatNumber(config.unitPriceCnyPerSecond, 2)} / 秒
                  </dd>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <dt className="text-[var(--text-muted)]">预计每片段</dt>
                  <dd className="font-mono font-semibold">
                    ¥{formatNumber(config.unitPriceCnyPerSecond * config.durationSeconds, 2)}
                  </dd>
                </div>
              </dl>
            </div>
            <div>
              <span className="text-[var(--text-muted)]">接口地址</span>
              <p className="mt-2 break-all rounded-lg bg-[var(--surface-raised)] p-3 font-mono text-xs">
                {config.baseUrl}
              </p>
            </div>
            <p className="text-xs leading-5 text-[var(--text-dim)]">
              成本按当前模型、分辨率和输出秒数估算，暂不支持修改，最终以 MiniMax 账单为准。API Key
              保存后立即生效，管理接口不会返回明文。
            </p>
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Qwen 控场模型</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-5 lg:grid-cols-[2fr_1fr]">
          <div className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm font-semibold text-[var(--text-muted)] sm:col-span-2">
                百炼 API Key
                <input
                  className={fieldClass}
                  type="password"
                  autoComplete="new-password"
                  value={qwenAPIKey}
                  onChange={(event) => setQwenAPIKey(event.target.value)}
                  placeholder={
                    qwenConfig?.apiKeyConfigured
                      ? "已配置；留空则保持不变"
                      : "请输入 DASHSCOPE API Key"
                  }
                />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)] sm:col-span-2">
                OpenAI 兼容 Base URL
                <input
                  className={fieldClass}
                  type="url"
                  value={qwenBaseURL}
                  onChange={(event) => setQwenBaseURL(event.target.value)}
                  placeholder="https://dashscope.aliyuncs.com/compatible-mode/v1"
                />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                Observer
                <input
                  className={fieldClass}
                  value={qwenConfig?.observerModel ?? "qwen3.8-flash"}
                  disabled
                />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                Director
                <input
                  className={fieldClass}
                  value={qwenConfig?.directorModel ?? "qwen3.8-max"}
                  disabled
                />
              </label>
            </div>
            <div aria-live="polite" className="min-h-5 text-sm">
              {qwenError ? (
                <span className="text-rose-600 dark:text-rose-300">{qwenError}</span>
              ) : qwenMessage ? (
                <span className="text-emerald-600 dark:text-emerald-300">{qwenMessage}</span>
              ) : null}
            </div>
            <Button
              variant="primary"
              disabled={qwenSaving || !qwenBaseURL.trim()}
              onClick={() => void saveQwen()}
            >
              <Save className="size-4" aria-hidden="true" />
              {qwenSaving ? "保存中…" : "保存 Qwen 配置"}
            </Button>
          </div>
          <div className="space-y-4 rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-4 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="inline-flex items-center gap-2 text-[var(--text-muted)]">
                <Eye className="size-4" />
                Observer
              </span>
              <Badge tone="accent">qwen3.8-flash</Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="inline-flex items-center gap-2 text-[var(--text-muted)]">
                <BrainCircuit className="size-4" />
                Director
              </span>
              <Badge tone="accent">qwen3.8-max</Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-[var(--text-muted)]">API Key</span>
              <Badge tone={qwenConfig?.apiKeyConfigured ? "success" : "danger"}>
                {qwenConfig?.apiKeyConfigured ? "已配置" : "未配置"}
              </Badge>
            </div>
            <p className="text-xs leading-5 text-[var(--text-dim)]">
              两种模型均关闭思考并使用严格 JSON Schema。Observer
              可读取最近弹幕和最近可用视频片段；Director 使用观察结果规划后续方向。API Key
              不会回显。
            </p>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Bilibili 弹幕输入</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-5 lg:grid-cols-[2fr_1fr]">
          <div className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                直播间房间号
                <input
                  className={fieldClass}
                  type="number"
                  min="1"
                  inputMode="numeric"
                  value={roomId}
                  onChange={(event) => setRoomId(event.target.value)}
                  placeholder="例如 123456"
                />
              </label>
              <label className="text-sm font-semibold text-[var(--text-muted)]">
                浏览器 Cookie
                <input
                  className={fieldClass}
                  type="password"
                  autoComplete="new-password"
                  value={cookie}
                  onChange={(event) => setCookie(event.target.value)}
                  placeholder={
                    bilibiliConfig?.cookieConfigured ? "已配置；留空则保持不变" : "粘贴完整 Cookie"
                  }
                />
              </label>
            </div>
            <p className="text-xs leading-5 text-[var(--text-dim)]">
              只读取该直播间的公开弹幕。Cookie 仅保存在当前服务内存中，接口不会返回明文；建议包含
              SESSDATA、bili_jct 与 buvid3。
            </p>
            <div aria-live="polite" className="min-h-5 text-sm">
              {bilibiliError ? (
                <span className="text-rose-600 dark:text-rose-300">{bilibiliError}</span>
              ) : bilibiliMessage ? (
                <span className="text-emerald-600 dark:text-emerald-300">{bilibiliMessage}</span>
              ) : null}
            </div>
            <Button
              variant="primary"
              disabled={bilibiliSaving || !roomId}
              onClick={() => void saveBilibili()}
            >
              <Save className="size-4" aria-hidden="true" />
              {bilibiliSaving ? "连接中…" : "保存并连接"}
            </Button>
            <div className="rounded-xl border border-dashed border-[var(--line-strong)] bg-[var(--surface-raised)] p-4">
              <div className="flex items-center gap-2 font-semibold text-[var(--text)]">
                <MessageCircleMore className="size-4 text-[var(--accent)]" aria-hidden="true" />
                本地模拟弹幕
              </div>
              <div className="mt-3 grid gap-3 sm:grid-cols-[160px_1fr_auto] sm:items-end">
                <label className="text-xs font-semibold text-[var(--text-muted)]">
                  昵称
                  <input
                    className={fieldClass}
                    value={mockUsername}
                    maxLength={40}
                    onChange={(event) => setMockUsername(event.target.value)}
                  />
                </label>
                <label className="text-xs font-semibold text-[var(--text-muted)]">
                  弹幕内容
                  <input
                    className={fieldClass}
                    value={mockText}
                    maxLength={200}
                    onChange={(event) => setMockText(event.target.value)}
                    placeholder="例如：看看桌上的杯子"
                  />
                </label>
                <Button
                  variant="secondary"
                  disabled={mockSaving || !mockText.trim()}
                  onClick={() => void sendMock()}
                >
                  <Send className="size-4" aria-hidden="true" />
                  {mockSaving ? "发送中…" : "发送"}
                </Button>
              </div>
              <div aria-live="polite" className="mt-3 min-h-5 text-xs">
                {mockError ? (
                  <span className="text-rose-600 dark:text-rose-300">{mockError}</span>
                ) : mockMessage ? (
                  <span className="text-emerald-600 dark:text-emerald-300">{mockMessage}</span>
                ) : (
                  <span className="text-[var(--text-dim)]">
                    无需连接 Bilibili，发送后可立即去 Flow 看板观察 20 秒窗口。
                  </span>
                )}
              </div>
            </div>
          </div>
          <div className="space-y-4 rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-4 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[var(--text-muted)]">弹幕连接</span>
              <Badge tone={bilibiliTone(bilibiliConfig?.status)}>
                <RadioTower className="size-3" />
                {bilibiliStatusLabel(bilibiliConfig?.status)}
              </Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-[var(--text-muted)]">Cookie</span>
              <Badge tone={bilibiliConfig?.cookieConfigured ? "success" : "warning"}>
                {bilibiliConfig?.cookieConfigured ? "已配置" : "未配置"}
              </Badge>
            </div>
            <div>
              <span className="text-[var(--text-muted)]">最近错误</span>
              <p className="mt-2 break-words rounded-lg bg-[var(--surface)] p-3 text-xs">
                {bilibiliConfig?.lastError ?? "无"}
              </p>
            </div>
            <p className="text-xs leading-5 text-[var(--text-dim)]">
              收到的弹幕进入最近 20 秒窗口，只影响尚未提交生成的后续片段，不会改写已计费任务。
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function bilibiliStatusLabel(status?: BilibiliConfig["status"]) {
  return (
    {
      disconnected: "未连接",
      connecting: "连接中",
      connected: "已连接",
      failed: "连接失败",
    } as const
  )[status ?? "disconnected"]
}

function bilibiliTone(
  status?: BilibiliConfig["status"],
): "success" | "warning" | "danger" | "neutral" {
  if (status === "connected") return "success"
  if (status === "connecting") return "warning"
  if (status === "failed") return "danger"
  return "neutral"
}
