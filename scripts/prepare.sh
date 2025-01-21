#!/bin/bash

# Установка зависимостей
go mod init myproject
go get -u github.com/lib/pq

# Подготовка базы данных
psql -U validator -d project-sem-1 -c "CREATE TABLE IF NOT EXISTS prices (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL, category VARCHAR(255) NOT NULL, price DECIMAL(10, 2) NOT NULL, create_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP);"
