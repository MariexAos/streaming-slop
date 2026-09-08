import { useMutation, useQuery } from "@tanstack/react-query"
import { fetchGuestStatus, sendGuestMessage } from "@/lib/guest-api"

export function useGuestStatus() {
  return useQuery({
    queryKey: ["guest-status"],
    queryFn: ({ signal }) => fetchGuestStatus(signal),
    refetchInterval: 2000,
  })
}
export function useGuestMessage() {
  return useMutation({ mutationFn: sendGuestMessage, retry: false })
}
