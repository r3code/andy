-- +goose Up
-- +goose StatementBegin
IF NOT EXISTS ( SELECT *
 FROM sys.schemas
 WHERE name = N'schema_a' )
 EXEC('CREATE SCHEMA [schema_a] AUTHORIZATION [dbo]');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF EXISTS ( SELECT *
 FROM sys.schemas
 WHERE name = N'schema_a' )
 EXEC('DROP SCHEMA [schema_a]');
-- +goose StatementEnd


-- +goose Up
-- +goose StatementBegin
CREATE TABLE schema_a.Inventory (id INT, name NVARCHAR(50), quantity INT);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE schema_a.Inventory;
-- +goose StatementEnd
