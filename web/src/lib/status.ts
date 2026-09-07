export function toneForStatus(
  status: string,
): "success" | "warning" | "danger" | "accent" | "neutral" {
  if (["running", "live", "normal", "ready", "playing"].includes(status)) return "success"
  if (["starting", "buffering", "recovering", "high", "critical", "fallback"].includes(status))
    return "warning"
  if (status === "failed") return "danger"
  if (["committed", "generating"].includes(status)) return "accent"
  return "neutral"
}

export function statusLabel(status: string) {
  return (
    (
      {
        stopped: "已停止",
        starting: "启动中",
        buffering: "缓冲中",
        running: "运行中",
        stopping: "停止中",
        recovering: "恢复中",
        failed: "失败",
        live: "直播中",
        normal: "正常",
        high: "高风险",
        critical: "严重",
        fallback: "备用模式",
        pause: "暂停",
        planned: "已规划",
        generating: "生成中",
        ready: "已就绪",
        committed: "已提交",
        playing: "播放中",
        played: "已播放",
      } as Record<string, string>
    )[status] ?? status
  )
}

export function connectionLabel(status: string) {
  return (
    (
      { connecting: "连接中", open: "已连接", reconnecting: "重连中", closed: "已关闭" } as Record<
        string,
        string
      >
    )[status] ?? status
  )
}

export function reasonLabel(reason: string | null) {
  if (!reason) return "无"
  return (
    (
      { operator: "人工操作", automatic: "自动触发", critical: "缓冲严重不足" } as Record<
        string,
        string
      >
    )[reason] ?? reason
  )
}
