package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type App struct {
	DB *sql.DB
}

type GuestbookEntry struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Email   string    `json:"email"`
	Website string    `json:"website"`
	Message string    `json:"message"`
	Date    time.Time `json:"date"`
}

type createEntryRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Website string `json:"website"`
	Message string `json:"message"`
}

func main() {
	log.Println("imba")

	db, err := sql.Open("sqlite", "file:app.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	app := &App{DB: db}

	if err := app.migrate(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("GET /guestbook", app.listEntries)
	mux.HandleFunc("POST /guestbook", app.createEntry)
	mux.HandleFunc("GET /guestbook/{id}", app.getEntry)
	mux.HandleFunc("DELETE /guestbook/{id}", app.deleteEntry)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

func (a *App) migrate() error {
	_, err := a.DB.Exec(`
	CREATE TABLE IF NOT EXISTS guestbook (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		website TEXT NOT NULL,
		message TEXT NOT NULL,
		date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	return err
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func (a *App) listEntries(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id, name, email, website, message, date FROM guestbook ORDER BY id DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []GuestbookEntry
	for rows.Next() {
		var e GuestbookEntry
		if err := rows.Scan(&e.ID, &e.Name, &e.Email, &e.Website, &e.Message, &e.Date); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entries = append(entries, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (a *App) createEntry(w http.ResponseWriter, r *http.Request) {
	var req createEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Email == "" || req.Website == "" || req.Message == "" {
		http.Error(w, "all fields are required", http.StatusBadRequest)
		return
	}

	res, err := a.DB.Exec(
		`INSERT INTO guestbook (name, email, website, message) VALUES (?, ?, ?, ?)`,
		req.Name, req.Email, req.Website, req.Message,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var entry GuestbookEntry
	err = a.DB.QueryRow(
		`SELECT id, name, email, website, message, date FROM guestbook WHERE id = ?`,
		id,
	).Scan(&entry.ID, &entry.Name, &entry.Email, &entry.Website, &entry.Message, &entry.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (a *App) getEntry(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var entry GuestbookEntry
	err = a.DB.QueryRow(
		`SELECT id, name, email, website, message, date FROM guestbook WHERE id = ?`,
		id,
	).Scan(&entry.ID, &entry.Name, &entry.Email, &entry.Website, &entry.Message, &entry.Date)

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (a *App) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, err := a.DB.Exec(`DELETE FROM guestbook WHERE id = ?`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	n, err := res.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

