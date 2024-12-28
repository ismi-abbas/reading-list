package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var tmpl *template.Template
var db *sql.DB

type Reading struct {
	Id          int64
	Url         string
	Title       string
	Description string
	Source      string
	Type        ReadingType
	AddDate     string
	AddTime     string
}

type ReadingType string

const (
	Article ReadingType = "article"
	Book    ReadingType = "book"
	Video   ReadingType = "video"
)

type ReadingForm struct {
	Id          int64
	Url         string
	Title       string
	Description string
	Source      string
	Type        ReadingType
	AddDate     string
	AddTime     string
}

func init() {
	tmpl, _ = template.ParseGlob("templates/*.html")
}

func initDb() {
	godotenv.Load()

	var err error

	database := os.Getenv("TURSO_DATABASE_URL")
	token := os.Getenv("TURSO_AUTH_TOKEN")
	url := database + "?authToken=" + token

	db, err = sql.Open("libsql", url)

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db %s: %s", url, err)
		os.Exit(1)
	}
	fmt.Println("Connected to database")

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Create readings table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS readings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		source TEXT,
		type TEXT,
		add_date DATE DEFAULT CURRENT_DATE,
		add_time TIME DEFAULT CURRENT_TIME
	)`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Insert test data if table is empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM readings").Scan(&count)
	if err != nil {
		log.Printf("Error checking table count: %v", err)
		return
	}

	if count == 0 {
		log.Println("Inserting test data...")
		testData := []struct {
			url, title, description, source, readingType string
		}{
			{
				url:         "https://go.dev/doc",
				title:       "Go Documentation",
				description: "Official Go programming language documentation",
				source:      "go.dev",
				readingType: string(Article),
			},
			{
				url:         "https://github.com/ismi-abbas/reading-list",
				title:       "Reading List Project",
				description: "A simple reading list application built with Go",
				source:      "GitHub",
				readingType: string(Article),
			},
		}

		for _, data := range testData {
			_, err := db.Exec(
				"INSERT INTO readings (url, title, description, source, type) VALUES (?, ?, ?, ?, ?)",
				data.url, data.title, data.description, data.source, data.readingType,
			)
			if err != nil {
				log.Printf("Error inserting test data: %v", err)
			}
		}
	}
}

func main() {
	initDb()
	defer db.Close()

	gRouter := mux.NewRouter()
	gRouter.HandleFunc("/", Homepage)
	gRouter.HandleFunc("/getReadingList", FetchReadings).Methods("GET")
	gRouter.HandleFunc("/addReading", AddReading).Methods("POST")
	gRouter.HandleFunc("/newReadingForm", AddReadingForm)
	gRouter.HandleFunc("/getReadingUpdateForm/{id}", EditReadingForm)
	gRouter.HandleFunc("/readings/{id}/delete", DeleteReading).Methods("DELETE")

	err := http.ListenAndServe(":8080", gRouter)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func Homepage(w http.ResponseWriter, r *http.Request) {
	types := []string{"Article", "Blog Post", "Documentation", "Book", "Tutorial"}
	tmpl.ExecuteTemplate(w, "index.html", types)
}

func FetchReadings(w http.ResponseWriter, r *http.Request) {
	readings, err := GetReadings(db)
	if err != nil {
		log.Printf("Error fetching readings: %v", err)
		http.Error(w, "Failed to fetch readings", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "readingList", readings)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func GetReadings(db *sql.DB) ([]Reading, error) {
	query := "SELECT * FROM readings"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var readings []Reading

	for rows.Next() {
		var reading Reading
		rowErr := rows.Scan(
			&reading.Id,
			&reading.Url,
			&reading.Title,
			&reading.Description,
			&reading.Source,
			&reading.Type,
			&reading.AddDate,
			&reading.AddTime,
		)

		if rowErr != nil {
			return nil, rowErr
		}
		readings = append(readings, reading)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return readings, nil
}

func DeleteReading(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	query := "DELETE FROM readings WHERE id = ?"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Printf("Error preparing delete statement: %v", err)
		http.Error(w, "Failed to delete reading", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(id)
	if err != nil {
		log.Printf("Error executing delete: %v", err)
		http.Error(w, "Failed to delete reading", http.StatusInternalServerError)
		return
	}

	readings, err := GetReadings(db)
	if err != nil {
		log.Printf("Error fetching readings after delete: %v", err)
		http.Error(w, "Failed to fetch readings", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "readingList", readings)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func AddReading(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	title := r.FormValue("title")
	description := r.FormValue("description")
	readingType := r.FormValue("type")
	source := r.FormValue("source")

	if url == "" || title == "" {
		http.Error(w, "URL and title are required", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO readings (url, title, description, type, source) VALUES (?, ?, ?, ?, ?)"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Printf("Error preparing insert statement: %v", err)
		http.Error(w, "Failed to add reading", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(url, title, description, readingType, source)
	if err != nil {
		log.Printf("Error executing insert: %v", err)
		http.Error(w, "Failed to add reading", http.StatusInternalServerError)
		return
	}

	readings, err := GetReadings(db)
	if err != nil {
		log.Printf("Error fetching readings after insert: %v", err)
		http.Error(w, "Failed to fetch readings", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "readingList", readings)

	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func AddReadingForm(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "addReadingForm", nil)
}

func EditReadingForm(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	tmpl.ExecuteTemplate(w, "editReadingForm", id)
}
