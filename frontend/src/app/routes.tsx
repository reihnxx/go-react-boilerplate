import { lazy, type ComponentType } from 'react'
import type { RouteObject } from 'react-router'

import { GuestOnly, RequireActiveRole, RequireAuth, RequirePermission } from '@/features/auth/guards'
import { LoginPage } from '@/features/auth/pages/LoginPage'
import { AppLayout, type RouteHandle } from '@/layouts/AppLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { ForbiddenPage } from '@/pages/ForbiddenPage'
import { HomeRedirect } from '@/pages/HomeRedirect'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { RouteErrorPage } from '@/pages/RouteErrorPage'

// Pages are code-split; the layouts render them inside <Suspense>.
const page = <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K) =>
  lazy(() => load().then((m) => ({ default: m[name] })))

const ChooseRolePage = page(() => import('@/features/auth/pages/ChooseRolePage'), 'ChooseRolePage')
const ForgotPasswordPage = page(() => import('@/features/auth/pages/ForgotPasswordPage'), 'ForgotPasswordPage')
const ResetPasswordPage = page(() => import('@/features/auth/pages/ResetPasswordPage'), 'ResetPasswordPage')
const RegisterPage = page(() => import('@/features/auth/pages/RegisterPage'), 'RegisterPage')
const DashboardPage = page(() => import('@/features/dashboard/DashboardPage'), 'DashboardPage')
const UsersPage = page(() => import('@/features/users/UsersPage'), 'UsersPage')
const RolesPage = page(() => import('@/features/roles/RolesPage'), 'RolesPage')
const RolePermissionsPage = page(() => import('@/features/roles/RolePermissionsPage'), 'RolePermissionsPage')
const MenusPage = page(() => import('@/features/menus/MenusPage'), 'MenusPage')
const ProfilePage = page(() => import('@/features/profile/ProfilePage'), 'ProfilePage')

const title = (t: string): RouteHandle => ({ title: t })

/*
 * Each path below must match the `path` of its menu row in the database, and
 * RequirePermission must use the same menu slug as the backend route guard.
 */
export const routes: RouteObject[] = [
  {
    errorElement: <RouteErrorPage />,
    children: [
      {
        element: <GuestOnly />,
        children: [
          {
            element: <AuthLayout />,
            children: [
              { path: '/login', element: <LoginPage /> },
              { path: '/forgot-password', element: <ForgotPasswordPage /> },
              { path: '/reset-password', element: <ResetPasswordPage /> },
              { path: '/register', element: <RegisterPage /> },
            ],
          },
        ],
      },
      {
        element: <RequireAuth />,
        children: [
          { element: <AuthLayout />, children: [{ path: '/choose-role', element: <ChooseRolePage /> }] },
          {
            element: <RequireActiveRole />,
            children: [
              {
                element: <AppLayout />,
                children: [
                  { index: true, element: <HomeRedirect /> },
                  {
                    path: '/dashboard',
                    handle: title('Beranda'),
                    element: (
                      <RequirePermission menu="dashboard">
                        <DashboardPage />
                      </RequirePermission>
                    ),
                  },
                  {
                    path: '/users',
                    handle: title('Pengguna'),
                    element: (
                      <RequirePermission menu="users">
                        <UsersPage />
                      </RequirePermission>
                    ),
                  },
                  {
                    path: '/roles',
                    handle: title('Otoritas'),
                    children: [
                      {
                        index: true,
                        element: (
                          <RequirePermission menu="roles">
                            <RolesPage />
                          </RequirePermission>
                        ),
                      },
                      {
                        path: ':id/permissions',
                        handle: title('Hak akses'),
                        element: (
                          <RequirePermission menu="roles">
                            <RolePermissionsPage />
                          </RequirePermission>
                        ),
                      },
                    ],
                  },
                  {
                    path: '/menus',
                    handle: title('Manajemen Menu'),
                    element: (
                      <RequirePermission menu="menus">
                        <MenusPage />
                      </RequirePermission>
                    ),
                  },
                  { path: '/profile', handle: title('Profil'), element: <ProfilePage /> },
                  { path: '/403', handle: title('Akses ditolak'), element: <ForbiddenPage /> },
                  { path: '*', handle: title('Tidak ditemukan'), element: <NotFoundPage /> },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
]
