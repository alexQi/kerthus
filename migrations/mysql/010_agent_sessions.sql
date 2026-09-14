CREATE TABLE IF NOT EXISTS agent_sessions (
  id CHAR(32) NOT NULL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  tenant_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  state_json JSON NOT NULL,
  expires_at BIGINT NOT NULL,
  lease_token CHAR(32) NOT NULL DEFAULT '',
  lease_expires_at BIGINT NOT NULL DEFAULT 0,
  INDEX idx_agent_session_owner (tenant_id, user_id, app_id),
  INDEX idx_agent_session_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
