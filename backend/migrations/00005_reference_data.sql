-- Reference data required in every environment. IDs are fixed so later
-- migrations and code can refer to them. No users are created here; use
-- `api create-admin` for the first administrator.

-- +goose Up
INSERT INTO actions (code, label, sort_order) VALUES
    ('view', 'Lihat', 10),
    ('create', 'Tambah', 20),
    ('update', 'Ubah', 30),
    ('delete', 'Hapus', 40),
    ('download', 'Unduh', 50);

INSERT INTO roles (id, name, slug, is_super_admin, is_system) VALUES
    ('00000000-0000-7000-8000-000000000001', 'Super Admin', 'super-admin', true, true),
    ('00000000-0000-7000-8000-000000000002', 'User', 'user', false, true);

INSERT INTO menu_groups (id, name, sort_order) VALUES
    ('00000000-0000-7000-8000-000000000101', 'Beranda', 10),
    ('00000000-0000-7000-8000-000000000102', 'Manajemen', 20),
    ('00000000-0000-7000-8000-000000000103', 'Referensi', 30),
    ('00000000-0000-7000-8000-000000000104', 'Data', 40);

INSERT INTO menus (id, group_id, name, slug, path, icon, sort_order) VALUES
    ('00000000-0000-7000-8000-000000000201', '00000000-0000-7000-8000-000000000101', 'Beranda', 'dashboard', '/dashboard', 'layout-dashboard', 10),
    ('00000000-0000-7000-8000-000000000202', '00000000-0000-7000-8000-000000000101', 'Profil', 'profile', '/profile', 'user', 20),
    ('00000000-0000-7000-8000-000000000203', '00000000-0000-7000-8000-000000000102', 'Pengguna', 'users', '/users', 'users', 10),
    ('00000000-0000-7000-8000-000000000204', '00000000-0000-7000-8000-000000000102', 'Manajemen Menu', 'menus', '/menus', 'list-tree', 20),
    ('00000000-0000-7000-8000-000000000205', '00000000-0000-7000-8000-000000000102', 'Otoritas', 'roles', '/roles', 'shield', 30);

-- Default "User" role: dashboard + profile, all actions (same as the Laravel seeder).
INSERT INTO role_permissions (role_id, menu_id, action)
SELECT '00000000-0000-7000-8000-000000000002', m.id, a.code
FROM menus m CROSS JOIN actions a
WHERE m.slug IN ('dashboard', 'profile');

-- +goose Down
DELETE FROM role_permissions;
DELETE FROM menus WHERE id IN (
    '00000000-0000-7000-8000-000000000201', '00000000-0000-7000-8000-000000000202',
    '00000000-0000-7000-8000-000000000203', '00000000-0000-7000-8000-000000000204',
    '00000000-0000-7000-8000-000000000205');
DELETE FROM menu_groups WHERE id IN (
    '00000000-0000-7000-8000-000000000101', '00000000-0000-7000-8000-000000000102',
    '00000000-0000-7000-8000-000000000103', '00000000-0000-7000-8000-000000000104');
DELETE FROM roles WHERE id IN (
    '00000000-0000-7000-8000-000000000001', '00000000-0000-7000-8000-000000000002');
DELETE FROM actions;
