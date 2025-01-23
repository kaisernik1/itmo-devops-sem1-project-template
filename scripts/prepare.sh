#!/bin/bash

go mod init myproject
go get -u github.com/lib/pq

export PGPASSWORD=val1dat0r

echo "Waiting for PostgreSQL to become available..."
until psql -h localhost -U validator -d project-sem-1 -c 'SELECT 1'; do
  sleep 1
done

psql -h localhost -U validator -d project-sem-1 -c "CREATE TABLE IF NOT EXISTS prices (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL, category VARCHAR(255) NOT NULL, price DECIMAL(10, 2) NOT NULL, create_date DATE);"
