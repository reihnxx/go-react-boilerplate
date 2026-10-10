import './index.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router'

import { Providers } from '@/app/providers'
import { createQueryClient, wireApiHandlers } from '@/app/query-client'
import { routes } from '@/app/routes'
import { appName } from '@/lib/config'

document.title = appName

const queryClient = createQueryClient()
wireApiHandlers(queryClient)
const router = createBrowserRouter(routes)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Providers client={queryClient}>
      <RouterProvider router={router} />
    </Providers>
  </StrictMode>,
)
