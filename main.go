package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

var tmpl *template.Template
var db *sql.DB

type reading struct {
	id          int64
	url         string
	title       string
	description string
	added_date  string
	added_time  string
}

func init() {
	tmpl, _ = template.ParseGlob("templates/*.html")
}

func initDb() {
	var err error
	db, err = sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/reading_list")
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	gRouter := mux.NewRouter()

	gRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "index.html", nil)
	})

	http.ListenAndServe(":8080", gRouter)

}
