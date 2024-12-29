package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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
	Status      ReadingStatus
	AddDate     string
	AddTime     string
}

type ReadingStatus string

const (
	ToBeRead ReadingStatus = "to-be-read"
	Halfway  ReadingStatus = "halfway"
	Unread   ReadingStatus = "unread"
	Read     ReadingStatus = "read"
)

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
		status TEXT,
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
	unreadCount, _ := GetCountByStatus(db, Unread)
	readCount, _ := GetCountByStatus(db, Read)
	toBeReadCount, _ := GetCountByStatus(db, ToBeRead)
	halfwayCount, _ := GetCountByStatus(db, Halfway)
	allCount := unreadCount + readCount + toBeReadCount + halfwayCount
	tmpl.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"types":         types,
		"unreadCount":   unreadCount,
		"readCount":     readCount,
		"toBeReadCount": toBeReadCount,
		"halfwayCount":  halfwayCount,
		"allCount":      allCount,
	})
}

func GetCountByStatus(db *sql.DB, status ReadingStatus) (int, error) {
	query := "SELECT COUNT(*) FROM readings WHERE status = ?"
	var count int
	err := db.QueryRow(query, status).Scan(&count)
	return count, err
}

func FetchReadings(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	fmt.Printf("status: %s\n", status)

	var readings []Reading
	var err error

	if status == "all" || status == "" {
		readings, err = GetReadings(db)
	} else {
		readings, err = GetReadingsByStatus(db, status)
	}

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

func GetReadingsByStatus(db *sql.DB, status string) ([]Reading, error) {
	query := "SELECT id, url, title, description, source, type, status, add_date, add_time FROM readings WHERE status = ?"
	rows, err := db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []Reading
	for rows.Next() {
		var reading Reading
		err := rows.Scan(
			&reading.Id,
			&reading.Url,
			&reading.Title,
			&reading.Description,
			&reading.Source,
			&reading.Type,
			&reading.Status,
			&reading.AddDate,
			&reading.AddTime,
		)
		if err != nil {
			return nil, err
		}
		readings = append(readings, reading)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return readings, nil
}

func GetReadings(db *sql.DB) ([]Reading, error) {
	query := "SELECT id, url, title, description, source, type, status, add_date, add_time FROM readings"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []Reading
	for rows.Next() {
		var reading Reading
		err := rows.Scan(
			&reading.Id,
			&reading.Url,
			&reading.Title,
			&reading.Description,
			&reading.Source,
			&reading.Type,
			&reading.Status,
			&reading.AddDate,
			&reading.AddTime,
		)
		if err != nil {
			return nil, err
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
	description := r.FormValue("description")
	readingType := r.FormValue("type")
	source := r.FormValue("source")

	generatedTitle := generateTitleWithLlama3(description)

	if url == "" {
		http.Error(w, "URL and title are required", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO readings (url, title, description, type, source, status) VALUES (?, ?, ?, ?, ?, ?)"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Printf("Error preparing insert statement: %v", err)
		http.Error(w, "Failed to add reading", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(url, generatedTitle, description, readingType, source, Unread)
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

var systemPrompt = `You are an expert summarizer with a unique ability to distill complex information into concise, descriptive titles. Your role is to take any input text and create a single, clear title that captures its essence. The title should be informative yet brief, ideally between 3-8 words. \n Rules: 1. Always respond with exactly one title\n 2. Never include additional explanations\n 3. Focus on the main theme or key message\n 4. Use clear, descriptive language\n 5. Avoid unnecessary articles (a, an, the)\n 6. Keep character count under 60`

func checkURL(url string) bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func getAvailableURL() string {
	urls := []string{
		"http://localhost:11434",
		os.Getenv("LLAMA_API_URL_WINDOWS"),
		os.Getenv("LLAMA_API_URL_LINUX"),
	}

	for _, baseURL := range urls {
		if baseURL == "" {
			continue
		}

		url := baseURL + "/api/chat"
		if checkURL(url) {
			return baseURL
		}
	}

	return ""
}

func generateTitleWithLlama3(content string) string {
	baseURL := getAvailableURL()
	if baseURL == "" {
		fmt.Println("No available Llama API endpoints")
		return ""
	}

	url := baseURL + "/api/chat"
	method := "POST"

	fmt.Println("Using URL:", url)
	fmt.Println("content:", content)

	// Escape special characters in the content
	escapedContent := strings.ReplaceAll(content, "\\", "\\\\")
	escapedContent = strings.ReplaceAll(escapedContent, "\"", "\\\"")
	escapedContent = strings.ReplaceAll(escapedContent, "\n", "\\n")
	escapedContent = strings.ReplaceAll(escapedContent, "\r", "\\r")
	escapedContent = strings.ReplaceAll(escapedContent, "\t", "\\t")

	// create json payload
	payload := strings.NewReader(`{
		"model": "llama3",
		"messages": [
			{
				"role": "system",
				"content": "` + systemPrompt + `"
			},
			{
				"role": "user",
				"content": "` + escapedContent + `"
			}
		],
		"stream": false
	}`)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return ""
	}
	req.Header.Add("Content-Type", "application/json")

	// Try up to 3 times
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		res, err := client.Do(req)
		if err != nil {
			fmt.Printf("Attempt %d failed: %v\n", i+1, err)
			if i < maxRetries-1 {
				time.Sleep(time.Second * 2) // Wait 2 seconds before retrying
				continue
			}
			return ""
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Println("Error reading response:", err)
			return ""
		}

		// Parse the JSON response
		var response struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			fmt.Println("Error parsing response:", err)
			return ""
		}

		if response.Message.Content != "" {
			return response.Message.Content
		}

		// If we got an empty response and have more retries, try again
		if i < maxRetries-1 {
			time.Sleep(time.Second * 2)
			continue
		}
	}

	return ""
}
