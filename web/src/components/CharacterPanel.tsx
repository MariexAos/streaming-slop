import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  attachJob,
  fetchBudget,
  fetchCharacter,
  fetchCharacters,
  fetchUnresolved,
  publishCharacter,
  selectCharacter,
} from "@/lib/characters"
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card"
import { Button } from "./ui/button"

export function CharacterPanel() {
  const client = useQueryClient()
  const current = useQuery({ queryKey: ["character"], queryFn: fetchCharacter })
  const versions = useQuery({ queryKey: ["characters"], queryFn: fetchCharacters })
  const budget = useQuery({ queryKey: ["budget"], queryFn: fetchBudget, refetchInterval: 5000 })
  const pending = useQuery({
    queryKey: ["unresolved"],
    queryFn: fetchUnresolved,
    refetchInterval: 5000,
  })
  const [first, setFirst] = useState<File>()
  const [last, setLast] = useState<File>()
  const [voice, setVoice] = useState<string>()
  const [description, setDescription] = useState<string>()
  const [jobs, setJobs] = useState<Record<string, string>>({})
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["characters"] }),
      client.invalidateQueries({ queryKey: ["character"] }),
      client.invalidateQueries({ queryKey: ["unresolved"] }),
    ])
  }
  const select = useMutation({ mutationFn: selectCharacter, onSuccess: refresh })
  const upload = useMutation({ mutationFn: publishCharacter, onSuccess: refresh })
  const attach = useMutation({ mutationFn: attachJob, onSuccess: refresh })
  const error =
    select.error ??
    upload.error ??
    attach.error ??
    current.error ??
    versions.error ??
    budget.error ??
    pending.error
  const profile = current.data
  return (
    <Card>
      <CardHeader>
        <CardTitle>人物与运行额度</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-[var(--text-muted)]">
          人物版本在开播时固定；切换后用于下一场直播。
        </p>
        {error && (
          <p role="alert" className="text-rose-600">
            {error.message}
          </p>
        )}
        {profile && (
          <>
            <label className="block">
              人物版本
              <select
                aria-label="人物版本"
                className="ml-3 rounded border p-2"
                value={profile.id}
                disabled={select.isPending}
                onChange={(event) => select.mutate(event.target.value)}
              >
                {versions.data?.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name} · {item.id.slice(0, 8)}
                  </option>
                ))}
              </select>
            </label>
            <div className="grid grid-cols-2 gap-3">
              {["start", "end"].map((frame) => (
                <img
                  key={frame}
                  src={`/api/v1/characters/${profile.id}/${frame}`}
                  alt={frame === "start" ? "当前人物首帧" : "当前人物尾帧"}
                  className="aspect-video w-full rounded-xl object-cover"
                />
              ))}
            </div>
            <details className="rounded-xl border border-[var(--line)] p-4">
              <summary className="min-h-11 text-sm font-semibold">编辑人物与音色</summary>
              <div className="space-y-4 pt-3">
                <label className="block">
                  人物描述
                  <textarea
                    aria-label="人物描述"
                    className="mt-1 block w-full rounded border p-2"
                    value={description ?? profile.description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </label>
                <label className="block">
                  固定音色 ID（留空使用视频原声）
                  <input
                    aria-label="固定音色 ID"
                    className="mt-1 block w-full rounded border p-2"
                    value={voice ?? profile.voiceId ?? ""}
                    onChange={(event) => setVoice(event.target.value)}
                  />
                </label>
                <p className="text-sm">配置音色后，短台词会使用固定音色合成；口型仍需人工验收。</p>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label>
                    新首帧
                    <input
                      type="file"
                      accept="image/png,image/jpeg"
                      onChange={(event) => setFirst(event.target.files?.[0])}
                    />
                  </label>
                  <label>
                    新尾帧
                    <input
                      type="file"
                      accept="image/png,image/jpeg"
                      onChange={(event) => setLast(event.target.files?.[0])}
                    />
                  </label>
                </div>
                <Button
                  disabled={upload.isPending}
                  onClick={() => {
                    upload.mutate({
                      profile: {
                        ...profile,
                        voiceId: voice ?? profile.voiceId,
                        description: description ?? profile.description,
                      },
                      first,
                      last,
                    })
                  }}
                >
                  保存新版本
                </Button>
                {upload.isSuccess && (
                  <p role="status">新版本已保存，可在上方选择。当前直播继续使用原版本。</p>
                )}
              </div>
            </details>
          </>
        )}
        {budget.data && (
          <p>
            总额度 ¥{(budget.data.limitMicros / 1e6).toFixed(2)} · 已结算 ¥
            {(budget.data.chargedMicros / 1e6).toFixed(4)} · 在途预占 ¥
            {(budget.data.reservedMicros / 1e6).toFixed(4)}
          </p>
        )}
        {(pending.data?.some((item) => !item.jobId) || pending.error) && (
          <p className="text-sm">
            提交结果不明时，请从供应商控制台找到对应任务，再绑定任务编号。绑定不会重新生成。
          </p>
        )}
        {pending.data
          ?.filter((item) => !item.jobId)
          .map((item) => (
            <div key={item.id} className="flex flex-wrap items-center gap-2">
              <span className="text-xs">{item.id}</span>
              <input
                aria-label={`任务编号 ${item.id}`}
                className="rounded border p-2"
                value={jobs[item.id] ?? ""}
                onChange={(event) => setJobs({ ...jobs, [item.id]: event.target.value })}
              />
              <Button
                disabled={!jobs[item.id]?.trim() || attach.isPending}
                onClick={() => attach.mutate({ id: item.id, jobId: jobs[item.id]?.trim() ?? "" })}
              >
                绑定任务
              </Button>
            </div>
          ))}
      </CardContent>
    </Card>
  )
}
