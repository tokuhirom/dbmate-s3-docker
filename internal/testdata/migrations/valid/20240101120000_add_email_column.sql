-- migrate:up
ALTER TABLE test_table ADD COLUMN email VARCHAR(255) UNIQUE;

-- migrate:down
ALTER TABLE test_table DROP COLUMN email;
