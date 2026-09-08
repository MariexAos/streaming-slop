import { fetchBudget } from "@/lib/budget"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  attachJob,
  fetchCharacter,
  fetchCharacters,
  fetchUnresolved,
  publishCharacter,
  selectCharacter,
} from "@/lib/characters"

export function useCharacterManagement() {
  const client = useQueryClient()
  const current = useQuery({ queryKey: ["character"], queryFn: fetchCharacter })
  const versions = useQuery({ queryKey: ["characters"], queryFn: fetchCharacters })
  const budget = useQuery({ queryKey: ["budget"], queryFn: fetchBudget, refetchInterval: 5000 })
  const pending = useQuery({
    queryKey: ["unresolved"],
    queryFn: fetchUnresolved,
    refetchInterval: 5000,
  })
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["characters"] }),
      client.invalidateQueries({ queryKey: ["readiness"] }),
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
  return { current, versions, budget, pending, select, upload, attach, error }
}
