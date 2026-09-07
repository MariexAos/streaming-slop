import { useEffect } from "react"
import { queryOptions, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query"
import { fetchSnapshot, openOpsEvents } from "@/lib/api"
import type { OpsSnapshot } from "@/lib/schema"
import { useOpsStore } from "@/store/ops"
import { snapshotKey } from "./client"

export function acceptSnapshot(client: QueryClient, snapshot: OpsSnapshot) {
  const accepted = client.setQueryData<OpsSnapshot>(snapshotKey, snapshot)
  if (accepted) useOpsStore.getState().applySnapshot(accepted)
  return accepted ?? snapshot
}

export function snapshotOptions(client: QueryClient) {
  return queryOptions({
    queryKey: snapshotKey,
    queryFn: async ({ signal }) => {
      const snapshot = await fetchSnapshot(signal)
      return acceptSnapshot(client, snapshot)
    },
  })
}

export function useSnapshot() {
  return useQuery(snapshotOptions(useQueryClient()))
}

export function useOpsEvents() {
  const client = useQueryClient()
  useEffect(() => {
    const { setConnection, setDataError } = useOpsStore.getState()
    setConnection("connecting")
    let opened = false
    const close = openOpsEvents({
      onOpen: () => {
        setConnection("open")
        if (opened) void client.invalidateQueries({ queryKey: snapshotKey })
        opened = true
      },
      onSnapshot: (snapshot) => {
        acceptSnapshot(client, snapshot)
        setDataError(null)
      },
      onError: (message) => {
        setConnection("reconnecting")
        if (message) setDataError(message)
      },
    })
    return () => {
      close()
      setConnection("closed")
    }
  }, [client])
}
