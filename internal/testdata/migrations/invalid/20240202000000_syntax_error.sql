-- migrate:up
CREATE TABLE invalid_syntax (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    CONSTRAINT INVALID SYNTAX HERE
);

-- migrate:down
DROP TABLE IF EXISTS invalid_syntax;
