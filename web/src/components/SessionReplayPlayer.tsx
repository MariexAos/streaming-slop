import { useImperativeHandle, useRef, useState, type Ref } from "react"
import { Maximize, Pause, Play, Volume2, VolumeX } from "lucide-react"
import type { SessionHistory } from "@/lib/schema"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

export type ReplayHandle = { seekSegment: (id: string) => void }

type Props = { segments: SessionHistory["segments"]; ref: Ref<ReplayHandle> }

export function SessionReplayPlayer({ segments, ref }: Props) {
  const video = useRef<HTMLVideoElement>(null)
  const container = useRef<HTMLDivElement>(null)
  const pendingTime = useRef(0)
  const shouldPlay = useRef(false)
  const [index, setIndex] = useState(0)
  const [time, setTime] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [rate, setRate] = useState(1)
  const [error, setError] = useState("")
  const [durations, setDurations] = useState<Record<string, number>>({})
  const chapters: (SessionHistory["segments"][number] & { offset: number; duration: number })[] = []
  let total = 0
  for (const segment of segments
    .filter((item) => item.playable)
    .sort((a, b) => a.sequence - b.sequence)) {
    const duration = durations[segment.id] ?? segment.endSeconds - segment.startSeconds
    chapters.push({ ...segment, offset: total, duration })
    total += duration
  }
  const current = chapters[index]
  const progress = current ? Math.min(current.offset + time, total) : 0

  const play = () => {
    const element = video.current
    if (!element) return
    shouldPlay.current = true
    void element.play().catch((reason: unknown) => {
      if (reason instanceof DOMException && reason.name === "AbortError") return
      shouldPlay.current = false
      setPlaying(false)
      setError("播放未能开始，请点击播放重试。")
    })
  }
  const seek = (nextIndex: number, seconds: number) => {
    const element = video.current
    pendingTime.current = seconds
    setTime(seconds)
    setError("")
    if (nextIndex === index && element && element.readyState >= 1) {
      element.currentTime = seconds
    } else {
      setIndex(nextIndex)
    }
  }
  useImperativeHandle(ref, () => ({
    seekSegment(id) {
      const nextIndex = chapters.findIndex((chapter) => chapter.id === id)
      if (nextIndex >= 0) seek(nextIndex, 0)
    },
  }))

  if (!current) {
    return (
      <div className="grid aspect-video place-items-center rounded-xl bg-black text-sm text-white/70">
        该直播没有可播放片段
      </div>
    )
  }
  return (
    <div ref={container} className="min-w-0 space-y-3 bg-[var(--panel-solid)]">
      <div className="relative aspect-video w-full overflow-hidden rounded-xl bg-black">
        <video
          ref={video}
          src={`/api/v1/ops/media/${current.id}`}
          aria-label="整场直播回放"
          className="absolute inset-0 block h-full w-full object-contain"
          playsInline
          preload="auto"
          muted={muted}
          onLoadedMetadata={(event) => {
            const element = event.currentTarget
            if (Number.isFinite(element.duration) && element.duration > 0) {
              setDurations((previous) => ({ ...previous, [current.id]: element.duration }))
            }
            element.currentTime = Math.min(pendingTime.current, element.duration)
            element.playbackRate = rate
            if (shouldPlay.current) play()
          }}
          onTimeUpdate={(event) => setTime(event.currentTarget.currentTime)}
          onPlay={() => setPlaying(true)}
          onPause={() => setPlaying(false)}
          onEnded={() => {
            if (index + 1 < chapters.length) {
              shouldPlay.current = true
              seek(index + 1, 0)
            } else {
              shouldPlay.current = false
              setPlaying(false)
            }
          }}
          onError={() => {
            setPlaying(false)
            setError(`片段 #${current.sequence} 加载失败，可重试或跳到其他片段。`)
          }}
        />
      </div>
      <div className="flex items-center justify-between gap-3 text-sm">
        <span className="min-w-0 truncate" title={current.direction.action}>
          当前片段 #{current.sequence} · {current.direction.action}
        </span>
        <span className="shrink-0 font-mono" aria-label="回放时间">
          {timestamp(progress)} / {timestamp(total)}
        </span>
      </div>
      <div>
        <input
          type="range"
          aria-label="整场播放进度"
          aria-valuetext={`${timestamp(progress)}，片段 #${current.sequence}`}
          min={0}
          max={total}
          step={0.1}
          value={progress}
          className="block w-full accent-[var(--accent)]"
          onChange={(event) => {
            const target = Number(event.target.value)
            const found = chapters.findIndex(
              (chapter) => target < chapter.offset + chapter.duration,
            )
            const nextIndex = found < 0 ? chapters.length - 1 : found
            seek(nextIndex, target - chapters[nextIndex].offset)
          }}
        />
        <div className="mt-1 flex h-8 gap-px" aria-label="回放片段">
          {chapters.map((chapter, chapterIndex) => (
            <Tooltip key={chapter.id}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  aria-label={`跳到片段 #${chapter.sequence}`}
                  aria-current={index === chapterIndex ? "true" : undefined}
                  style={{ flex: `${chapter.duration} 1 0%` }}
                  className={`min-w-0 overflow-hidden rounded text-xs focus-visible:outline-2 focus-visible:outline-[var(--accent)] ${index === chapterIndex ? "bg-[var(--accent)] text-white" : "bg-[var(--surface-hover)] text-[var(--text-muted)]"}`}
                  onClick={() => seek(chapterIndex, 0)}
                >
                  #{chapter.sequence}
                </button>
              </TooltipTrigger>
              <TooltipContent>
                <p>
                  片段 #{chapter.sequence} · {timestamp(chapter.offset)}–
                  {timestamp(chapter.offset + chapter.duration)}
                </p>
                <p>{chapter.direction.action}</p>
                {chapter.direction.dialogue && <p>{chapter.direction.dialogue}</p>}
              </TooltipContent>
            </Tooltip>
          ))}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          size="icon"
          aria-label={playing ? "暂停回放" : "播放回放"}
          onClick={() => {
            if (playing) {
              shouldPlay.current = false
              video.current?.pause()
            } else {
              if (progress >= total) seek(0, 0)
              play()
            }
          }}
        >
          {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
        </Button>
        <Button
          size="icon"
          aria-label={muted ? "开启声音" : "静音"}
          onClick={() => setMuted(!muted)}
        >
          {muted ? <VolumeX className="size-4" /> : <Volume2 className="size-4" />}
        </Button>
        <select
          aria-label="播放速度"
          value={rate}
          className="min-h-11 rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] px-2 text-sm"
          onChange={(event) => {
            const value = Number(event.target.value)
            setRate(value)
            if (video.current) video.current.playbackRate = value
          }}
        >
          {[0.5, 1, 1.5, 2].map((value) => (
            <option key={value} value={value}>
              {value}×
            </option>
          ))}
        </select>
        <Button
          size="icon"
          aria-label="全屏回放"
          onClick={() => {
            void container.current
              ?.requestFullscreen()
              .catch(() => setError("无法进入全屏，请重试。"))
          }}
        >
          <Maximize className="size-4" />
        </Button>
        {error && (
          <p role="alert" className="text-sm text-rose-600">
            {error}{" "}
            <button
              type="button"
              className="underline"
              onClick={() => {
                setError("")
                pendingTime.current = time
                shouldPlay.current = true
                video.current?.load()
              }}
            >
              重试
            </button>
          </p>
        )}
      </div>
    </div>
  )
}

function timestamp(seconds: number) {
  const value = Math.floor(seconds)
  const hours = Math.floor(value / 3600)
  const minutes = Math.floor((value % 3600) / 60)
  const rest = String(value % 60).padStart(2, "0")
  return hours ? `${hours}:${String(minutes).padStart(2, "0")}:${rest}` : `${minutes}:${rest}`
}
