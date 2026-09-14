ALTER TABLE `tenants` ADD COLUMN `agent_providers` JSON NOT NULL AFTER `agent_enabled`;
