CREATE TABLE invalid_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- migrate:down
DROP TABLE IF EXISTS invalid_table;
