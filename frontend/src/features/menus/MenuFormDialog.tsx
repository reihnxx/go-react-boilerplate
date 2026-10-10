import { zodResolver } from '@hookform/resolvers/zod'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { FormField } from '@/components/form-field'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { handleFormError } from '@/lib/errors'
import { cn } from '@/lib/utils'
import type { Menu } from '@/types/api'

import { useCreateMenu, useMenuGroups, useParentMenus, useUpdateMenu } from './api'
import { IconPicker } from './IconPicker'

const schema = z
  .object({
    name: z.string().trim().min(1, 'Nama menu wajib diisi.').max(100, 'Nama menu maksimal 100 karakter.'),
    slug: z
      .string()
      .trim()
      .refine((v) => v === '' || /^[a-z][a-z0-9_]{1,62}$/.test(v), 'Huruf kecil, angka dan garis bawah; diawali huruf.'),
    type: z.enum(['main', 'child']),
    group_id: z.string().nullable(),
    parent_id: z.string().nullable(),
    path: z
      .string()
      .trim()
      .max(200, 'Path maksimal 200 karakter.')
      .refine((v) => v === '' || /^\/[A-Za-z0-9/_:.-]*$/.test(v), 'Path harus diawali "/" (contoh: /laporan).'),
    icon: z.string(),
    sort_order: z.number().int().min(0, 'Urutan tidak boleh negatif.').nullable(),
    is_active: z.boolean(),
  })
  // Same rules as the API (and the Laravel original).
  .superRefine((v, ctx) => {
    if (v.type === 'main') {
      if (!v.group_id) ctx.addIssue({ code: 'custom', path: ['group_id'], message: 'Grup menu wajib diisi untuk menu utama.' })
      if (!v.icon) ctx.addIssue({ code: 'custom', path: ['icon'], message: 'Ikon wajib diisi untuk menu utama.' })
    } else {
      if (!v.parent_id) ctx.addIssue({ code: 'custom', path: ['parent_id'], message: 'Menu induk wajib diisi untuk submenu.' })
      if (!v.path) ctx.addIssue({ code: 'custom', path: ['path'], message: 'Path wajib diisi untuk submenu.' })
    }
  })
type Values = z.infer<typeof schema>

const FIELDS = ['name', 'slug', 'type', 'group_id', 'parent_id', 'path', 'icon', 'sort_order', 'is_active'] as const

export function MenuFormDialog({ open, onOpenChange, menu }: { open: boolean; onOpenChange: (o: boolean) => void; menu?: Menu }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl">
        {open && <MenuForm key={menu?.id ?? 'new'} menu={menu} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function MenuForm({ menu, onDone }: { menu?: Menu; onDone: () => void }) {
  const groups = useMenuGroups()
  const parents = useParentMenus()
  const create = useCreateMenu()
  const update = useUpdateMenu()
  const pending = create.isPending || update.isPending

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: menu?.name ?? '',
      slug: menu?.slug ?? '',
      type: menu?.type ?? 'main',
      group_id: menu?.group_id ?? null,
      parent_id: menu?.parent_id ?? null,
      path: menu?.path ?? '',
      icon: menu?.icon ?? '',
      sort_order: menu?.sort_order ?? null,
      is_active: menu?.is_active ?? true,
    },
  })
  const { errors } = form.formState
  const type = useWatch({ control: form.control, name: 'type' })

  const onSubmit = form.handleSubmit((values) => {
    const opts = {
      onSuccess: () => {
        toast.success(menu ? 'Menu diperbarui' : 'Menu ditambahkan', {
          description: menu ? undefined : 'Atur hak akses menu ini di halaman Otoritas.',
        })
        onDone()
      },
      onError: (err: unknown) => handleFormError(err, form.setError, FIELDS),
    }
    if (menu) update.mutate({ id: menu.id, ...values }, opts)
    else create.mutate(values, opts)
  })

  return (
    <form noValidate onSubmit={onSubmit}>
      <DialogHeader>
        <DialogTitle>{menu ? 'Ubah menu' : 'Tambah menu'}</DialogTitle>
        <DialogDescription>Menu utama tampil di grup sidebar; submenu tampil di bawah menu utama (maksimal 2 tingkat).</DialogDescription>
      </DialogHeader>
      <FieldGroup className="py-4">
        <Controller
          control={form.control}
          name="type"
          render={({ field }) => (
            <div className="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1" role="radiogroup" aria-label="Jenis menu">
              {(['main', 'child'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  role="radio"
                  aria-checked={field.value === t}
                  onClick={() => field.onChange(t)}
                  className={cn(
                    'rounded-md py-1.5 text-sm font-medium text-muted-foreground',
                    field.value === t && 'bg-background text-foreground shadow-sm',
                  )}
                >
                  {t === 'main' ? 'Menu utama' : 'Submenu'}
                </button>
              ))}
            </div>
          )}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Nama menu" htmlFor="menu-name" error={errors.name?.message} required>
            <Input id="menu-name" autoComplete="off" aria-invalid={!!errors.name} {...form.register('name')} />
          </FormField>
          <FormField
            label="Slug (kunci hak akses)"
            htmlFor="menu-slug"
            error={errors.slug?.message}
            description={menu ? 'Tidak dapat diubah.' : 'Kosongkan untuk dibuat otomatis dari nama.'}
          >
            <Input id="menu-slug" autoComplete="off" disabled={!!menu} aria-invalid={!!errors.slug} {...form.register('slug')} />
          </FormField>
        </div>

        {type === 'main' ? (
          <FormField label="Grup menu" htmlFor="menu-group" error={errors.group_id?.message} required>
            <Controller
              control={form.control}
              name="group_id"
              render={({ field }) => (
                <Select value={field.value ?? ''} onValueChange={field.onChange}>
                  <SelectTrigger id="menu-group" className="w-full" aria-invalid={!!errors.group_id}>
                    <SelectValue placeholder="Pilih grup" />
                  </SelectTrigger>
                  <SelectContent>
                    {groups.data?.map((g) => (
                      <SelectItem key={g.id} value={g.id}>
                        {g.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </FormField>
        ) : (
          <FormField label="Menu induk" htmlFor="menu-parent" error={errors.parent_id?.message} required>
            <Controller
              control={form.control}
              name="parent_id"
              render={({ field }) => (
                <Select value={field.value ?? ''} onValueChange={field.onChange}>
                  <SelectTrigger id="menu-parent" className="w-full" aria-invalid={!!errors.parent_id}>
                    <SelectValue placeholder="Pilih menu induk" />
                  </SelectTrigger>
                  <SelectContent>
                    {parents.data
                      ?.filter((p) => p.id !== menu?.id)
                      .map((p) => (
                        <SelectItem key={p.id} value={p.id}>
                          {p.name} {p.group_name && <span className="text-muted-foreground">· {p.group_name}</span>}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              )}
            />
          </FormField>
        )}

        <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
          <FormField
            label="Path"
            htmlFor="menu-path"
            error={errors.path?.message}
            required={type === 'child'}
            description={type === 'main' ? 'Kosongkan bila menu ini hanya induk dari submenu.' : 'Harus sama dengan route di frontend.'}
          >
            <Input id="menu-path" placeholder="/laporan" autoComplete="off" aria-invalid={!!errors.path} {...form.register('path')} />
          </FormField>
          <FormField label="Urutan" htmlFor="menu-order" error={errors.sort_order?.message} description="Otomatis bila kosong.">
            <Input
              id="menu-order"
              type="number"
              min={0}
              aria-invalid={!!errors.sort_order}
              {...form.register('sort_order', { setValueAs: (v) => (v === '' || v === null ? null : Number(v)) })}
            />
          </FormField>
        </div>

        <FormField label="Ikon" htmlFor="menu-icon" error={errors.icon?.message} required={type === 'main'}>
          <Controller
            control={form.control}
            name="icon"
            render={({ field }) => <IconPicker value={field.value} onChange={field.onChange} invalid={!!errors.icon} />}
          />
        </FormField>

        <Controller
          control={form.control}
          name="is_active"
          render={({ field }) => (
            <Field orientation="horizontal">
              <Switch id="menu-active" checked={field.value} onCheckedChange={field.onChange} />
              <FieldLabel htmlFor="menu-active" className="font-normal">
                Menu aktif (nonaktif = disembunyikan dan tidak dapat diakses selain oleh Super Admin)
              </FieldLabel>
            </Field>
          )}
        />
      </FieldGroup>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button type="submit" disabled={pending}>
          {pending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </form>
  )
}
