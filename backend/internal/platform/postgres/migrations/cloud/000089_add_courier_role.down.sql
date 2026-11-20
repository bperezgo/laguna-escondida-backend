-- Migration: add_courier_role (down)
-- Version: 000089

DELETE FROM roles r WHERE r.name = 'courier'
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.role_id = r.id);
