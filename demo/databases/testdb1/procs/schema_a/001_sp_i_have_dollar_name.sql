CREATE
OR ALTER PROCEDURE [schema_a].[sp_i_have_dollar_name] AS MERGE [schema_a].[Inventory] AS tar USING [schema_a].[Inventory] AS src ON tar.[id] = src.[id]
WHEN NOT MATCHED THEN
INSERT ([id], [name], [quantity])
VALUES (src.[id], src.[name], src.[quantity]) OUTPUT $action,
    inserted.*,
    deleted.*;
