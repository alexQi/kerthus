ALTER TABLE `tenants`
  ADD COLUMN `agent_provider` VARCHAR(64) NOT NULL DEFAULT '' AFTER `expires_at`,
  ADD COLUMN `agent_model` VARCHAR(191) NOT NULL DEFAULT '' AFTER `agent_provider`,
  ADD COLUMN `agent_endpoint` VARCHAR(255) NOT NULL DEFAULT '' AFTER `agent_model`,
  ADD COLUMN `agent_api_key` TEXT NOT NULL AFTER `agent_endpoint`,
  ADD COLUMN `agent_enabled` TINYINT NOT NULL DEFAULT 0 AFTER `agent_api_key`;
