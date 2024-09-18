#!/bin/bash
echo First start mssql container with docker-compose

: "${PROJECT_NAME:=andy}"
CONTAINER="${PROJECT_NAME}_db_1"
: "${DB_NAME=testdb1}"

create_db() {
    sudo podman exec -it ${CONTAINER} /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P "Passw0rd" -b -Q "CREATE DATABASE ${DB_NAME};"
}

check_container_present() {
  sudo podman container exists ${CONTAINER}
}

check_container_present;
if check_container_present; then
    echo "OK"
else
  echo "ERROR: No podman container named ${CONTAINER}. Check with podman"
  exit 1
fi

create_db;
if create_db; then
  echo "Failed to create DB."
  exit 1
else
    echo "INFO: DB Created or Exists."
fi

echo "Create schema_a"
sudo podman exec -it ${CONTAINER} /opt/mssql-tools/bin/sqlcmd -S localhost -d ${DB_NAME} -U SA -P "Passw0rd" -Q "IF NOT EXISTS (SELECT * FROM sys.schemas  WHERE name = N'schema_a' ) EXEC('CREATE SCHEMA [schema_a]');"

echo "Create table schema_a.Inventory"
sudo podman exec -it ${CONTAINER} /opt/mssql-tools/bin/sqlcmd -S localhost -d ${DB_NAME} -U SA -P "Passw0rd" -Q "CREATE TABLE schema_a.Inventory (id INT, name NVARCHAR(50), quantity INT);"
