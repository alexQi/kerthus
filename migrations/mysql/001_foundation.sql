CREATE TABLE IF NOT EXISTS platform_lock (id INT PRIMARY KEY) ENGINE=InnoDB;
INSERT IGNORE INTO platform_lock(id) VALUES (1);

CREATE TABLE IF NOT EXISTS `users` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone VARCHAR(64) NOT NULL DEFAULT '',
  email VARCHAR(191) NOT NULL DEFAULT '',
  name VARCHAR(191) NOT NULL,
  avatar TEXT NOT NULL,
  sex INT NOT NULL DEFAULT 0,
  status INT NOT NULL DEFAULT 1,
  password_hash VARCHAR(255) NOT NULL,
  platform_admin BOOLEAN NOT NULL DEFAULT FALSE,
  auth_version BIGINT NOT NULL DEFAULT 1,
  phone_key VARCHAR(64) GENERATED ALWAYS AS (NULLIF(phone,'')) STORED,
  email_key VARCHAR(191) GENERATED ALWAYS AS (NULLIF(email,'')) STORED,
  UNIQUE KEY uq_user_phone (phone_key),
  UNIQUE KEY uq_user_email (email_key),
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `tenants` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(191) NOT NULL,
  logo TEXT NOT NULL,
  contact_person VARCHAR(191) NOT NULL DEFAULT '',
  contact_phone VARCHAR(64) NOT NULL DEFAULT '',
  contact_email VARCHAR(191) NOT NULL DEFAULT '',
  credit_code VARCHAR(64) NOT NULL DEFAULT '',
  address_json TEXT NOT NULL,
  address_detail TEXT NOT NULL,
  description TEXT NOT NULL,
  status INT NOT NULL DEFAULT 1,
  verify_status INT NOT NULL DEFAULT 0,
  expires_at BIGINT NOT NULL DEFAULT 0,
  bootstrap_version BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `members` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  status INT NOT NULL DEFAULT 1,
  default_app_id BIGINT NOT NULL DEFAULT 0,
  default_unit_id BIGINT NOT NULL DEFAULT 0,
  default_org_id BIGINT NOT NULL DEFAULT 0,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uq_member (tenant_id,user_id),
  KEY ix_member_user (user_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `orgs` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  unit_id BIGINT NOT NULL DEFAULT 0,
  type VARCHAR(16) NOT NULL,
  name VARCHAR(191) NOT NULL,
  short_name VARCHAR(191) NOT NULL DEFAULT '',
  status INT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  remark TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  KEY ix_org_tree (tenant_id,parent_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `positions` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  org_id BIGINT NOT NULL,
  name VARCHAR(191) NOT NULL,
  status INT NOT NULL DEFAULT 1,
  remark TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  KEY ix_position_tenant (tenant_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (org_id) REFERENCES orgs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `member_orgs` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  org_id BIGINT NOT NULL,
  UNIQUE KEY uq_member_org (tenant_id,user_id,org_id),
  FOREIGN KEY (tenant_id,user_id) REFERENCES members(tenant_id,user_id),
  FOREIGN KEY (org_id) REFERENCES orgs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `member_positions` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  position_id BIGINT NOT NULL,
  UNIQUE KEY uq_member_position (tenant_id,user_id,position_id),
  FOREIGN KEY (tenant_id,user_id) REFERENCES members(tenant_id,user_id),
  FOREIGN KEY (position_id) REFERENCES positions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `apps` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(191) NOT NULL,
  version VARCHAR(64) NOT NULL DEFAULT '',
  status INT NOT NULL DEFAULT 1,
  service_key VARCHAR(191) NOT NULL DEFAULT '',
  route_prefix VARCHAR(191) NOT NULL DEFAULT '',
  home VARCHAR(191) NOT NULL DEFAULT '',
  frontend_entry VARCHAR(191) NOT NULL DEFAULT '',
  resource_version BIGINT NOT NULL DEFAULT 1,
  managed BOOLEAN NOT NULL DEFAULT FALSE,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `resources` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  app_id BIGINT NOT NULL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  code VARCHAR(191) NOT NULL,
  name VARCHAR(191) NOT NULL,
  type VARCHAR(16) NOT NULL,
  path VARCHAR(191) NOT NULL DEFAULT '',
  component VARCHAR(191) NOT NULL DEFAULT '',
  icon VARCHAR(191) NOT NULL DEFAULT '',
  meta_json TEXT NOT NULL,
  status INT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  is_data_access BOOLEAN NOT NULL DEFAULT FALSE,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uq_resource_code (app_id,code),
  FOREIGN KEY (app_id) REFERENCES apps(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `operations` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  app_id BIGINT NOT NULL,
  resource_id BIGINT NOT NULL,
  operation_id VARCHAR(191) NOT NULL,
  method VARCHAR(12) NOT NULL,
  path VARCHAR(191) NOT NULL,
  group_name VARCHAR(191) NOT NULL DEFAULT '',
  action VARCHAR(191) NOT NULL DEFAULT '',
  UNIQUE KEY uq_app_operation (app_id,operation_id),
  UNIQUE KEY uq_app_route (app_id,method,path),
  FOREIGN KEY (app_id) REFERENCES apps(id),
  FOREIGN KEY (resource_id) REFERENCES resources(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `entitlements` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  status INT NOT NULL DEFAULT 1,
  expires_at BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uq_entitlement (tenant_id,app_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (app_id) REFERENCES apps(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `tenant_resources` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  resource_id BIGINT NOT NULL,
  UNIQUE KEY uq_tenant_resource (tenant_id,app_id,resource_id),
  FOREIGN KEY (tenant_id,app_id) REFERENCES entitlements(tenant_id,app_id),
  FOREIGN KEY (resource_id) REFERENCES resources(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `roles` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  code VARCHAR(191) NOT NULL,
  name VARCHAR(191) NOT NULL,
  remark TEXT NOT NULL,
  status INT NOT NULL DEFAULT 1,
  administrator BOOLEAN NOT NULL DEFAULT FALSE,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uq_role_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `role_members` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  UNIQUE KEY uq_role_member (tenant_id,role_id,user_id),
  FOREIGN KEY (role_id) REFERENCES roles(id),
  FOREIGN KEY (tenant_id,user_id) REFERENCES members(tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `grants` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  resource_id BIGINT NOT NULL,
  data_scope INT NOT NULL DEFAULT 5,
  UNIQUE KEY uq_grant (tenant_id,role_id,app_id,resource_id),
  FOREIGN KEY (role_id) REFERENCES roles(id),
  FOREIGN KEY (resource_id) REFERENCES resources(id),
  CHECK(data_scope BETWEEN 0 AND 5)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `audits` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_id BIGINT NOT NULL,
  tenant_id BIGINT NOT NULL DEFAULT 0,
  app_id BIGINT NOT NULL DEFAULT 0,
  action VARCHAR(191) NOT NULL,
  target_id BIGINT NOT NULL DEFAULT 0,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  KEY ix_audit_tenant (tenant_id,created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `files` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  object_key VARCHAR(255) NOT NULL UNIQUE,
  content_type VARCHAR(191) NOT NULL,
  size BIGINT NOT NULL,
  status INT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  KEY ix_file_owner (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `districts` (
  id BIGINT PRIMARY KEY,
  parent_id BIGINT NOT NULL DEFAULT 0,
  name VARCHAR(191) NOT NULL,
  KEY ix_district_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
