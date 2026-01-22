-- migrate:up
CREATE TABLE invalid_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
