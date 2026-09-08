import { useState } from "react"
import type { Provider } from "@/lib/services-api"
import { useModelServices } from "@/queries/services"
import { saveResult } from "@/queries/config"
import { Button } from "./ui/button"
export function ProviderCredential({
  provider,
  configured,
}: {
  provider: Provider
  configured: boolean
}) {
  const [key, setKey] = useState("")
  const { credential } = useModelServices()
  const name = provider === "fal" ? "fal" : "MiniMax"
  return (
    <form
      className="space-y-3 rounded-xl border p-4"
      onSubmit={(event) => {
        event.preventDefault()
        void (async () => {
          if (await saveResult(credential.mutateAsync({ provider, apiKey: key.trim() }))) setKey("")
        })()
      }}
    >
      <p className="font-medium">{name}</p>
      <p className="text-sm text-neutral-500">
        {configured ? "凭据已保存，未验证连接" : "尚未配置凭据"} ·{" "}
        {provider === "fal" ? "用于视频生成" : "用于视频生成、文本与视觉推理、语音"}
      </p>
      <label className="block text-sm">
        {name} API Key
        <input
          type="password"
          autoComplete="new-password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className="mt-2 block w-full rounded-xl border p-3"
          placeholder="输入新凭据，留空不会修改"
          disabled={credential.isPending}
        />
      </label>
      <Button disabled={!key.trim() || credential.isPending} type="submit">
        {credential.isPending ? "保存中…" : `保存 ${name} 凭据`}
      </Button>
      {credential.error && <p role="alert">{credential.error.message}</p>}
      {credential.isSuccess && (
        <p role="status" className="text-sm">
          凭据已保存。视频服务读取对应供应商的凭据；文本推理在下次开播时加载。
        </p>
      )}
    </form>
  )
}
