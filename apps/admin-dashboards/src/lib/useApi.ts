import { useState, useEffect, useCallback } from 'react'

// Flat result shape (not a discriminated union): callers destructure
// `{ status, data, refetch }`, and destructuring a discriminated union drops the
// per-variant `data` field (TS2339). `data`/`message` are therefore optional and
// callers guard on `status`. (update.md L1 — admin-dashboards now type-checks.)
export interface QueryResult<T> {
  status: 'loading' | 'ok' | 'error'
  data?: T
  message?: string
  refetch: () => void
}

export function useQuery<T>(fetcher: () => Promise<T>, deps: unknown[] = []): QueryResult<T> {
  const [state, setState] = useState<{ status: 'loading' | 'ok' | 'error'; data?: T; message?: string }>({
    status: 'loading',
  })
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick(t => t + 1), [])

  useEffect(() => {
    let cancelled = false
    setState({ status: 'loading' })
    fetcher()
      .then(data => { if (!cancelled) setState({ status: 'ok', data }) })
      .catch(err => { if (!cancelled) setState({ status: 'error', message: err.message ?? 'Error' }) })
    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps])

  return { ...state, refetch }
}

// Auto-polling variant — refetches every `intervalMs` milliseconds.
export function usePoll<T>(fetcher: () => Promise<T>, intervalMs: number): QueryResult<T> {
  const q = useQuery(fetcher)

  useEffect(() => {
    const id = setInterval(q.refetch, intervalMs)
    return () => clearInterval(id)
  }, [q.refetch, intervalMs])

  return q
}
