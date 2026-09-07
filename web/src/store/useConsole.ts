import { useEffect, useState } from "react"
import {
  fetchBilibiliConfig,
  fetchCurrentFlow,
  fetchFlowCatalog,
  fetchGenerationConfig,
  fetchQwenConfig,
  fetchSnapshot,
  openOpsEvents,
  sendMockDanmaku,
  updateBilibiliConfig,
  updateGenerationConfig,
  updateQwenConfig,
} from "@/lib/api"
import type {
  BilibiliConfig,
  CurrentFlow,
  FlowCatalog,
  GenerationConfig,
  QwenConfig,
} from "@/lib/schema"
import { useOpsStore } from "./ops"

export function useConsole() {
  const store = useOpsStore()
  const [view, setView] = useState<"operations" | "flow" | "history" | "cost" | "config">(
    "operations",
  )
  const [generationConfig, setGenerationConfig] = useState<GenerationConfig | null>(null)
  const [bilibiliConfig, setBilibiliConfig] = useState<BilibiliConfig | null>(null)
  const [qwenConfig, setQwenConfig] = useState<QwenConfig | null>(null)
  const [configSaving, setConfigSaving] = useState(false)
  const [configMessage, setConfigMessage] = useState<string | null>(null)
  const [configError, setConfigError] = useState<string | null>(null)
  const [bilibiliSaving, setBilibiliSaving] = useState(false)
  const [bilibiliMessage, setBilibiliMessage] = useState<string | null>(null)
  const [bilibiliError, setBilibiliError] = useState<string | null>(null)
  const [mockSaving, setMockSaving] = useState(false)
  const [mockMessage, setMockMessage] = useState<string | null>(null)
  const [mockError, setMockError] = useState<string | null>(null)
  const [qwenSaving, setQwenSaving] = useState(false)
  const [qwenMessage, setQwenMessage] = useState<string | null>(null)
  const [qwenError, setQwenError] = useState<string | null>(null)
  const [flowCatalog, setFlowCatalog] = useState<FlowCatalog | null>(null)
  const [currentFlow, setCurrentFlow] = useState<CurrentFlow | null>(null)
  const [flowError, setFlowError] = useState<string | null>(null)

  const { applySnapshot, setConnection, setDataError } = store

  useEffect(() => {
    const controller = new AbortController()
    let closeEvents: (() => void) | undefined
    let disposed = false

    const connect = async () => {
      try {
        const [snapshot, config, bilibili, qwen] = await Promise.all([
          fetchSnapshot(controller.signal),
          fetchGenerationConfig(controller.signal),
          fetchBilibiliConfig(controller.signal),
          fetchQwenConfig(controller.signal),
        ])
        if (disposed) return
        applySnapshot(snapshot)
        setGenerationConfig(config)
        setBilibiliConfig(bilibili)
        setQwenConfig(qwen)
        void Promise.all([fetchFlowCatalog(controller.signal), fetchCurrentFlow(controller.signal)])
          .then(([catalog, current]) => {
            if (!disposed) {
              setFlowCatalog(catalog)
              setCurrentFlow(current)
              setFlowError(null)
            }
          })
          .catch(() => {
            if (!disposed) setFlowError("Flow 运行数据暂不可用。")
          })
        closeEvents = openOpsEvents({
          onOpen: () => setConnection("open"),
          onSnapshot: applySnapshot,
          onError: (message) => {
            setConnection("reconnecting")
            if (message) setDataError(message)
          },
        })
      } catch (error) {
        if (disposed) return
        setConnection("reconnecting")
        setDataError(error instanceof Error ? error.message : "无法加载运行数据。")
      }
    }

    void connect()
    return () => {
      disposed = true
      controller.abort()
      closeEvents?.()
    }
  }, [applySnapshot, setConnection, setDataError])

  useEffect(() => {
    if (view !== "flow") return
    const refresh = () =>
      void fetchCurrentFlow()
        .then((value) => {
          setCurrentFlow(value)
          setFlowError(null)
        })
        .catch(() => setFlowError("Flow 运行数据暂不可用。"))
    refresh()
    const timer = window.setInterval(refresh, 5000)
    return () => window.clearInterval(timer)
  }, [view])

  useEffect(() => {
    if (view !== "config") return
    const refresh = () =>
      void fetchBilibiliConfig()
        .then(setBilibiliConfig)
        .catch(() => undefined)
    const timer = window.setInterval(refresh, 5000)
    return () => window.clearInterval(timer)
  }, [view])

  const saveGenerationConfig = async (
    value: Pick<GenerationConfig, "model" | "resolution" | "durationSeconds" | "ratio"> & {
      apiKey?: string
    },
  ) => {
    setConfigSaving(true)
    setConfigError(null)
    setConfigMessage(null)
    try {
      setGenerationConfig(await updateGenerationConfig(value))
      setConfigMessage("配置已保存，将用于后续新任务。")
      return true
    } catch (error) {
      setConfigError(error instanceof Error ? error.message : "配置保存失败。")
      return false
    } finally {
      setConfigSaving(false)
    }
  }
  const saveBilibiliConfig = async (value: { roomId: number; cookie?: string }) => {
    setBilibiliSaving(true)
    setBilibiliError(null)
    setBilibiliMessage(null)
    try {
      setBilibiliConfig(await updateBilibiliConfig(value))
      setBilibiliMessage("配置已保存，正在连接直播间弹幕。")
      return true
    } catch (error) {
      setBilibiliError(error instanceof Error ? error.message : "弹幕配置保存失败。")
      return false
    } finally {
      setBilibiliSaving(false)
    }
  }
  const sendMock = async (value: { username: string; text: string }) => {
    setMockSaving(true)
    setMockError(null)
    setMockMessage(null)
    try {
      await sendMockDanmaku(value)
      setMockMessage("模拟弹幕已进入最近 20 秒窗口。")
      return true
    } catch (error) {
      setMockError(error instanceof Error ? error.message : "模拟弹幕发送失败。")
      return false
    } finally {
      setMockSaving(false)
    }
  }
  const saveQwenConfig = async (value: { baseUrl: string; apiKey?: string }) => {
    setQwenSaving(true)
    setQwenError(null)
    setQwenMessage(null)
    try {
      setQwenConfig(await updateQwenConfig(value))
      setQwenMessage("Qwen 配置已保存，将立即用于后续观察和导演调用。")
      return true
    } catch (error) {
      setQwenError(error instanceof Error ? error.message : "Qwen 配置保存失败。")
      return false
    } finally {
      setQwenSaving(false)
    }
  }
  return {
    store,
    view,
    setView,
    generationConfig,
    bilibiliConfig,
    qwenConfig,
    configSaving,
    configMessage,
    configError,
    bilibiliSaving,
    bilibiliMessage,
    bilibiliError,
    mockSaving,
    mockMessage,
    mockError,
    qwenSaving,
    qwenMessage,
    qwenError,
    flowCatalog,
    currentFlow,
    flowError,
    saveGenerationConfig,
    saveBilibiliConfig,
    sendMock,
    saveQwenConfig,
  }
}
