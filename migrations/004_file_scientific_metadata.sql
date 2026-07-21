USE clivegformer;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='file_object' AND COLUMN_NAME='region_code'),
  'SELECT 1',
  'ALTER TABLE file_object ADD COLUMN region_code VARCHAR(32) NULL AFTER file_name'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='file_object' AND COLUMN_NAME='block_index'),
  'SELECT 1',
  'ALTER TABLE file_object ADD COLUMN block_index INT UNSIGNED NULL AFTER region_code'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='file_object' AND COLUMN_NAME='data_year'),
  'SELECT 1',
  'ALTER TABLE file_object ADD COLUMN data_year SMALLINT UNSIGNED NULL AFTER block_index'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='file_object' AND INDEX_NAME='idx_file_region_year_status'),
  'SELECT 1',
  'CREATE INDEX idx_file_region_year_status ON file_object(region_code,data_year,status)'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='file_object' AND INDEX_NAME='idx_file_year_region_status'),
  'SELECT 1',
  'CREATE INDEX idx_file_year_region_status ON file_object(data_year,region_code,status)'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
