USE clivegformer;

SET @analysis_generated_at_ddl = (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'analysis_result'
        AND COLUMN_NAME = 'generated_at'
    ),
    'SELECT 1',
    'ALTER TABLE analysis_result ADD COLUMN generated_at DATETIME(3) NULL AFTER last_accessed_at'
  )
);
PREPARE analysis_generated_at_statement FROM @analysis_generated_at_ddl;
EXECUTE analysis_generated_at_statement;
DEALLOCATE PREPARE analysis_generated_at_statement;

UPDATE analysis_result
SET generated_at = updated_at
WHERE status = 1 AND generated_at IS NULL;
