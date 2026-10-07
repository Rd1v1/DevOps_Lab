package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Note struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var ErrNotFound = errors.New("note not found")

type Store interface {
	Health(context.Context) error
	List(context.Context) ([]Note, error)
	Create(context.Context, Note) (int64, error)
	Get(context.Context, int64) (Note, error)
}

type handler struct{ store Store }

func NewHandler(store Store) http.Handler {
	h := &handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/notes", h.listNotes)
	mux.HandleFunc("POST /api/notes", h.addNote)
	mux.HandleFunc("GET /api/notes/{id}", h.getNote)
	return mux
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if h.store.Health(ctx) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "db": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "db": true})
}

func (h *handler) listNotes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	notes, err := h.store.List(ctx)
	if err != nil {
		h.dbError(w, err)
		return
	}
	if notes == nil {
		notes = []Note{}
	}
	writeJSON(w, http.StatusOK, notes)
}

func (h *handler) dbError(w http.ResponseWriter, err error) {
	// Do not log a DSN or database password.
	log.Print("database operation failed")
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
}

func (h *handler) addNote(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var note Note
	if err := decoder.Decode(&note); err != nil {
		var tooLarge *http.MaxBytesError
		code := http.StatusBadRequest
		if errors.As(err, &tooLarge) {
			code = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, code, map[string]string{"error": "invalid JSON or body too large"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected one JSON object"})
		return
	}
	note.Title = strings.TrimSpace(note.Title)
	if note.Title == "" || len([]rune(note.Title)) > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title must contain 1 to 200 characters"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, err := h.store.Create(ctx, note)
	if err != nil {
		h.dbError(w, err)
		return
	}
	w.Header().Set("Location", "/api/notes/"+strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *handler) getNote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	note, err := h.store.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		h.dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}
