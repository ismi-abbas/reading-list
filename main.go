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

	fmt.Printf("Database URL: %s\n", url)

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
		add_date DATE DEFAULT CURRENT_DATE,
		add_time TIME DEFAULT CURRENT_TIME
	)`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	initDb()
	defer db.Close()

	gRouter := mux.NewRouter()
	gRouter.HandleFunc("/", Homepage)
	gRouter.HandleFunc("/readings/{id}", ReadingDetails)
	gRouter.HandleFunc("/newReadingForm", addReadingForm)
	gRouter.HandleFunc("/getReadingUpdateForm/{id}", editReadingForm)
	gRouter.HandleFunc("/readings/{id}/delete", DeleteReading).Methods("DELETE")
	gRouter.HandleFunc("/addReading", AddReading).Methods("POST")
	gRouter.HandleFunc("/readings", fetchReadings)
	err := http.ListenAndServe(":8080", gRouter)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func Homepage(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func fetchReadings(w http.ResponseWriter, r *http.Request) {
	readings, _ := getReadings(db)
	tmpl.ExecuteTemplate(w, "readingList", readings)
}

func ReadingDetails(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "readingDetails.html", nil)
}

func getReadings(db *sql.DB) ([]Reading, error) {
	query := "SELECT * FROM readings"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var readings []Reading

	for rows.Next() {
		var reading Reading
		rowErr := rows.Scan(&reading.Id, &reading.Url, &reading.Title, &reading.Description, &reading.AddDate, &reading.AddTime)

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
		log.Fatal(err)
	}
	defer stmt.Close()
	_, executeErr := stmt.Exec(id)
	if executeErr != nil {
		log.Fatal(executeErr)
	}
	readings, _ := getReadings(db)
	tmpl.ExecuteTemplate(w, "readingList", readings)
}

func AddReading(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	title := r.FormValue("title")
	description := r.FormValue("description")
	password := r.FormValue("password")

	if password != os.Getenv("PASSWORD") {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	query := "INSERT INTO readings (url, title, description) VALUES (?, ?, ?)"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, executeErr := stmt.Exec(url, title, description)
	if executeErr != nil {
		log.Fatal(executeErr)
	}
	readings, _ := getReadings(db)
	tmpl.ExecuteTemplate(w, "readingList", readings)
}

func addReadingForm(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "addReadingForm", nil)
}

func editReadingForm(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	tmpl.ExecuteTemplate(w, "editReadingForm", id)
}
