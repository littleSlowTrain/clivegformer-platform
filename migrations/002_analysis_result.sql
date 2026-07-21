USE clivegformer;

CREATE TABLE IF NOT EXISTS analysis_result (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  file_id BIGINT UNSIGNED NOT NULL,
  analysis_type VARCHAR(32) NOT NULL,
  parameters_json JSON NOT NULL,
  cache_key CHAR(64) NOT NULL,
  cache_version VARCHAR(128) NOT NULL,
  model VARCHAR(128) NOT NULL DEFAULT '',
  result_json JSON NULL,
  provider VARCHAR(32) NOT NULL DEFAULT '',
  status TINYINT NOT NULL DEFAULT 0,
  lease_token CHAR(36) NOT NULL DEFAULT '',
  lease_expires_at DATETIME(3) NULL,
  expires_at DATETIME(3) NULL,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  created_by_user_id BIGINT UNSIGNED NOT NULL,
  hit_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_accessed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  generated_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_analysis_cache_key (cache_key),
  KEY idx_analysis_file (file_id, analysis_type),
  KEY idx_analysis_status_lease (status, lease_expires_at),
  CONSTRAINT fk_analysis_result_file FOREIGN KEY (file_id) REFERENCES file_object(id)
) ENGINE=InnoDB;
