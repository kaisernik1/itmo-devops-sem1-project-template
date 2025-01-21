package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "validator"
	dbPassword = "validat0r"
	dbName     = "project-sem-1"
)

type Price struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
}

type SummaryResponse struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

func main() {
	http.HandleFunc("/api/v0/prices", pricesHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handlePostRequest(w, r)
	} else if r.Method == http.MethodGet {
		handleGetRequest(w)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handlePostRequest(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Could not get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	zipReader, err := zip.NewReader(file, r.ContentLength)
	if err != nil {
		http.Error(w, "Failed to read zip file", http.StatusInternalServerError)
		return
	}

	totalItems := 0
	totalCategories := make(map[string]struct{})
	totalPrice := 0.0

	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName))
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	for _, file := range zipReader.File {
		fr, err := file.Open()
		if err != nil {
			http.Error(w, "Failed to open file in zip", http.StatusInternalServerError)
			return
		}

		reader := csv.NewReader(fr)
		records, err := reader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to read CSV file", http.StatusInternalServerError)
			return
		}

		for _, record := range records {
			if len(record) < 3 {
				continue
			}

			price := Price{
				Name:     record[0],
				Category: record[1],
			}

			_, err = fmt.Sscanf(record[2], "%f", &price.Price)
			if err != nil {
				continue
			}

			_, err = db.Exec("INSERT INTO prices (name, category, price) VALUES ($1, $2, $3)", price.Name, price.Category, price.Price)
			if err != nil {
				http.Error(w, "Database insert failed", http.StatusInternalServerError)
				return
			}

			totalItems++
			totalCategories[price.Category] = struct{}{}
			totalPrice += price.Price
		}
		fr.Close()
	}

	summary := SummaryResponse{
		TotalItems:      totalItems,
		TotalCategories: len(totalCategories),
		TotalPrice:      totalPrice,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func handleGetRequest(w http.ResponseWriter) {
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName))
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT name, category, price FROM prices")
	if err != nil {
		http.Error(w, "Failed to query database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	records := [][]string{{"Name", "Category", "Price"}}
	for rows.Next() {
		var p Price
		if err := rows.Scan(&p.Name, &p.Category, &p.Price); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		records = append(records, []string{p.Name, p.Category, fmt.Sprintf("%f", p.Price)})
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(records); err != nil {
		http.Error(w, "Failed to write CSV", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	zipFile, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create zip file", http.StatusInternalServerError)
		return
	}

	if _, err := zipFile.Write(buffer.Bytes()); err != nil {
		http.Error(w, "Failed to write zip file", http.StatusInternalServerError)
		return
	}
}
