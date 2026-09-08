import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { fetchBudget, saveBudget } from "@/lib/budget"

export function useSessionBudget() {
  const client = useQueryClient()
  const query = useQuery({ queryKey: ["budget"], queryFn: fetchBudget, refetchInterval: 5000 })
  const save = useMutation({
    mutationFn: saveBudget,
    retry: false,
    onSuccess: async () => {
      await client.cancelQueries({ queryKey: ["budget"] })
      await Promise.all([
        client.invalidateQueries({ queryKey: ["budget"] }),
        client.invalidateQueries({ queryKey: ["readiness"] }),
      ])
    },
  })
  return { query, save }
}
