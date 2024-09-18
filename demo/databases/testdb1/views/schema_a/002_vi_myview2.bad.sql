
CREATE OR ALTER VIEW schema_a.MyView2
AS SELECT * FROM schema_a.Inventory WHERE quantity > 20 !ERROR_HERE! 22
