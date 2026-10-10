import { Circle, Users } from 'lucide-react'
import { describe, expect, it } from 'vitest'

import { firstPath } from '@/features/auth/hooks'
import { safeNext } from '@/features/auth/guards'
import { isActivePath } from '@/layouts/AppSidebar'

import { initials } from './format'
import { getIcon } from './icons'

describe('safeNext', () => {
  it('accepts in-app paths only', () => {
    expect(safeNext('/users?page=2')).toBe('/users?page=2')
    expect(safeNext(null)).toBeNull()
    expect(safeNext('https://evil.example')).toBeNull()
    expect(safeNext('//evil.example')).toBeNull()
    expect(safeNext('/\\evil.example')).toBeNull()
    expect(safeNext('users')).toBeNull()
  })
})

describe('isActivePath', () => {
  it('matches the path and its sub-paths', () => {
    expect(isActivePath('/roles', '/roles')).toBe(true)
    expect(isActivePath('/roles/1/permissions', '/roles')).toBe(true)
    expect(isActivePath('/roles-archive', '/roles')).toBe(false)
    expect(isActivePath('/roles', null)).toBe(false)
  })
})

describe('firstPath', () => {
  it('returns the first navigable path, descending into children', () => {
    expect(firstPath(undefined)).toBeNull()
    expect(
      firstPath([
        {
          id: 'g',
          name: 'G',
          items: [
            {
              id: 'p',
              name: 'P',
              slug: 'p',
              path: null,
              icon: null,
              children: [{ id: 'c', name: 'C', slug: 'c', path: '/c', icon: null, children: [] }],
            },
          ],
        },
      ]),
    ).toBe('/c')
  })
})

describe('icons', () => {
  it('falls back to a neutral icon for unknown names', () => {
    expect(getIcon('users')).toBe(Users)
    expect(getIcon('does-not-exist')).toBe(Circle)
    expect(getIcon(null)).toBe(Circle)
  })
})

describe('initials', () => {
  it('uses up to two words', () => {
    expect(initials('Budi Santoso Putra')).toBe('BS')
    expect(initials('admin')).toBe('A')
    expect(initials('  ')).toBe('?')
  })
})
