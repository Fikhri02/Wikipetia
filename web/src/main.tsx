import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'

import './index.css'
import { router } from './router'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // The builder and player both read aggregates that change only when this
      // user changes them, so refetching on every window focus is noise.
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5_000,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
