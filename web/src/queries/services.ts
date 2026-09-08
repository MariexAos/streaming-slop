import { useQuery, useMutation, useQueryClient, useIsMutating } from "@tanstack/react-query"
import { fetchServices, saveServices, saveCredential, type ModelServices } from "@/lib/services-api"
const key = ["model-services"]
export function useModelServices() {
  const client = useQueryClient()
  const onSuccess = async (data: ModelServices) => {
    await client.cancelQueries({ queryKey: key })
    client.setQueryData(key, data)
    await client.invalidateQueries({ queryKey: ["readiness"] })
  }
  return {
    query: useQuery({ queryKey: key, queryFn: ({ signal }) => fetchServices(signal) }),
    save: useMutation({ mutationKey: key, mutationFn: saveServices, onSuccess, retry: false }),
    credential: useMutation({
      mutationKey: key,
      mutationFn: saveCredential,
      onSuccess,
      retry: false,
    }),
  }
}
export function useServicesSaving() {
  return useIsMutating({ mutationKey: key }) > 0
}
