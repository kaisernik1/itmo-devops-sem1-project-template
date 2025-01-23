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
    "strconv"
    "time"

    _ "github.com/lib/pq"
)

const (
    dbHost     = "localhost"
    dbPort     = 5432
    dbUser     = "validator"
    dbPassword = "val1dat0r"
    dbName     = "project-sem-1"
)

type Product struct {
    ID         int       `json:"id"`
    CreateDate time.Time `json:"create_date"`
    Name       string    `json:"name"`
    Category   string    `json:"category"`
    Price      float64   `json:"price"`
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

    db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        dbHost, dbPort, dbUser, dbPassword, dbName))
    if err != nil {
        http.Error(w, "Database connection failed", http.StatusInternalServerError)
        return
    }
    defer db.Close()

    totalItems := 0
    totalCategories := make(map[string]struct{})
    totalPrice := 0.0

    for _, file := range zipReader.File {
        fr, err := file.Open()
        if err != nil {
            http.Error(w, "Failed to open file in zip", http.StatusInternalServerError)
            return
        }
        defer fr.Close()

        reader := csv.NewReader(fr)
        records, err := reader.ReadAll()
        if err != nil {
            http.Error(w, "Failed to read CSV file", http.StatusInternalServerError)
            return
        }

        for _, record := range records {
            if len(record) < 5 {
                continue
            }

            id, _ := strconv.Atoi(record[0])
            createDate, _ := time.Parse("2006-01-02", record[1])
            name := record[2]
            category := record[3]
            price, _ := strconv.ParseFloat(record[4], 64)

            _, err = db.Exec("INSERT INTO prices (id, name, category, price, create_date) VALUES ($1, $2, $3, $4, $5)",
                id, name, category, price, createDate)
            if err != nil {
                http.Error(w, "Database insert failed", http.StatusInternalServerError)
                return
            }

            totalItems++
            totalCategories[category] = struct{}{}
            totalPrice += price
        }
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

    rows, err := db.Query("SELECT id, create_date, name, category, price FROM prices")
    if err != nil {
        http.Error(w, "Failed to query database", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var records [][]string
    records = append(records, []string{"ID", "Create Date", "Name", "Category", "Price"}) // Заголовки CSV

    for rows.Next() {
        var p Product
        if err := rows.Scan(&p.ID, &p.CreateDate, &p.Name, &p.Category, &p.Price); err != nil {
            http.Error(w, "Failed to scan row", http.StatusInternalServerError)
            return
        }
        records = append(records, []string{
            strconv.Itoa(p.ID), 
            p.CreateDate.Format("2006-01-02"), 
            p.Name, 
            p.Category, 
            fmt.Sprintf("%.2f", p.Price), 
        })
    }

    // Записываем CSV в буфер
    var buffer bytes.Buffer
    writer := csv.NewWriter(&buffer)
    if err := writer.WriteAll(records); err != nil {
        http.Error(w, "Failed to write CSV", http.StatusInternalServerError)
        return
    }

    // Установка заголовков для zip-архива
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