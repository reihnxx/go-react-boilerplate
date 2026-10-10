import { QueryClient } from '@tanstack/react-query'

import { resetSession } from '@/features/auth/hooks'
import { isApiError, setApiHandlers } from '@/lib/api'
import { queryKeys } from '@/lib/query-keys'

export function createQueryClient() {
  const qc = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // Retry only transient failures, never 4xx answers.
        retry: (count, err) => count < 2 && !(isApiError(err) && err.status >= 400 && err.status < 500),
      },
      mutations: { retry: false },
    },
  })
  return qc
}

/**
 * Session loss anywhere → forget the identity (guards then redirect to login).
 * Missing active role → reload "me" so the guard sends the user to role choice.
 */
export function wireApiHandlers(qc: QueryClient) {
  setApiHandlers({
    onUnauthenticated: () => {
      if (qc.getQueryData(queryKeys.me)) resetSession(qc, null)
    },
    onRoleSelectionRequired: () => {
      qc.invalidateQueries({ queryKey: queryKeys.me })
    },
  })
}
