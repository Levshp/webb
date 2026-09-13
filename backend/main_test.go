package main

import (
	"time"
	"net/http"
	"net/http/httptest"

	"database/sql"

	"github.com/DATA-DOG/go-sqlmock"

	"bytes"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApp_health(t *testing.T) {
	a := &App{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	a.health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if body := rr.Body.String(); body != "Alive" {
		t.Fatalf("expected body %q, got %q", "Alive", body)
	}
}

func TestApp_listEntries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	a := &App{DB: db}

	rows := sqlmock.NewRows([]string{"id", "name", "email", "website", "message", "date"}).
		AddRow(2, "Bob", "bob@example.com", "https://bob.com", "hello", time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)).
		AddRow(1, "Alice", "alice@example.com", "https://alice.com", "hi", time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))

	mock.ExpectQuery(`SELECT id, name, email, website, message, date FROM guestbook ORDER BY id DESC`).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/guestbook", nil)
	rr := httptest.NewRecorder()

	a.listEntries(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	wantCT := "application/json"
	if got := rr.Header().Get("Content-Type"); got != wantCT {
		t.Fatalf("Content-Type = %q, want %q", got, wantCT)
	}

	wantBody := `[{"id":2,"name":"Bob","email":"bob@example.com","website":"https://bob.com","message":"hello","date":"2026-09-13T00:00:00Z"},{"id":1,"name":"Alice","email":"alice@example.com","website":"https://alice.com","message":"hi","date":"2026-09-13T00:00:00Z"}]
`

	// If your JSON uses lowercase field names, replace wantBody accordingly.

	if got := rr.Body.String(); got != wantBody {
		t.Fatalf("body = %q, want %q", got, wantBody)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApp_listEntries_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	a := &App{DB: db}

	mock.ExpectQuery(`SELECT id, name, email, website, message, date FROM guestbook ORDER BY id DESC`).
		WillReturnError(sql.ErrConnDone)

	req := httptest.NewRequest(http.MethodGet, "/entries", nil)
	rr := httptest.NewRecorder()

	a.listEntries(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateEntry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	app := &App{DB: db}

	body := `{
		"name":"Alice",
		"email":"alice@example.com",
		"website":"https://alice.dev",
		"message":"hello world"
	}`

	mock.ExpectExec(`INSERT INTO guestbook \(name, email, website, message\) VALUES \(\?, \?, \?, \?\)`).
		WithArgs("Alice", "alice@example.com", "https://alice.dev", "hello world").
		WillReturnResult(sqlmock.NewResult(1, 1))

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"id", "name", "email", "website", "message", "date"}).
		AddRow(1, "Alice", "alice@example.com", "https://alice.dev", "hello world", now)

	mock.ExpectQuery(`SELECT id, name, email, website, message, date FROM guestbook WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodPost, "/guestbook", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	app.createEntry(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var got GuestbookEntry
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != 1 {
		t.Fatalf("expected ID 1, got %d", got.ID)
	}
	if got.Name != "Alice" || got.Email != "alice@example.com" || got.Website != "https://alice.dev" || got.Message != "hello world" {
		t.Fatalf("unexpected entry: %+v", got)
	}
	if got.Date.IsZero() {
		t.Fatal("expected non-zero date")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApp_getEntry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	a := &App{DB: db}

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"id", "name", "email", "website", "message", "date"}).
		AddRow(1, "Alice", "alice@example.com", "https://alice.dev", "hello world", now)

	mock.ExpectQuery(`SELECT id, name, email, website, message, date FROM guestbook WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/guestbook/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	a.getEntry(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}

	var got GuestbookEntry
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != 1 || got.Name != "Alice" || got.Email != "alice@example.com" || got.Website != "https://alice.dev" || got.Message != "hello world" {
		t.Fatalf("unexpected entry: %+v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApp_getEntry_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	a := &App{DB: db}

	mock.ExpectQuery(`SELECT id, name, email, website, message, date FROM guestbook WHERE id = \?`).
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/guestbook/99", nil)
	req.SetPathValue("id", "99")
	rr := httptest.NewRecorder()

	a.getEntry(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	if body := rr.Body.String(); body != "not found\n" {
		t.Fatalf("body = %q, want %q", body, "not found\n")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApp_getEntry_InvalidID(t *testing.T) {
	a := &App{}

	req := httptest.NewRequest(http.MethodGet, "/guestbook/abc", nil)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()

	a.getEntry(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	if body := rr.Body.String(); body != "invalid id\n" {
		t.Fatalf("body = %q, want %q", body, "invalid id\n")
	}
}


func TestApp_deleteEntry_Unauthorized(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	app := &App{DB: db, DeleteToken: "secret"}

	req := httptest.NewRequest(http.MethodDelete, "/guestbook/1", nil)
	rr := httptest.NewRecorder()

	app.deleteEntry(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestApp_deleteEntry_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	app := &App{DB: db, DeleteToken: "secret"}

	mock.ExpectExec(`DELETE FROM guestbook WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(http.MethodDelete, "/guestbook/1", nil)
	req.SetPathValue("id", "1")
	req.Header.Set("X-Delete-Token", "secret")

	rr := httptest.NewRecorder()
	app.deleteEntry(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApp_deleteEntry_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	app := &App{DB: db, DeleteToken: "secret"}

	mock.ExpectExec(`DELETE FROM guestbook WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req := httptest.NewRequest(http.MethodDelete, "/guestbook/1", nil)
	req.SetPathValue("id", "1")
	req.Header.Set("X-Delete-Token", "secret")

	rr := httptest.NewRecorder()
	app.deleteEntry(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

