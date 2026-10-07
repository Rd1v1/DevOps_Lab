package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notes/internal/api"
)

type fakeStore struct {
	notes   []api.Note
	failure bool
}

func (s *fakeStore) Health(context.Context) error {
	if s.failure {
		return errors.New("offline")
	}
	return nil
}
func (s *fakeStore) List(ctx context.Context) ([]api.Note, error) { return s.notes, s.Health(ctx) }
func (s *fakeStore) Create(ctx context.Context, n api.Note) (int64, error) {
	if err := s.Health(ctx); err != nil {
		return 0, err
	}
	n.ID = int64(len(s.notes) + 1)
	s.notes = append(s.notes, n)
	return n.ID, nil
}
func (s *fakeStore) Get(ctx context.Context, id int64) (api.Note, error) {
	if err := s.Health(ctx); err != nil {
		return api.Note{}, err
	}
	for _, n := range s.notes {
		if n.ID == id {
			return n, nil
		}
	}
	return api.Note{}, api.ErrNotFound
}
func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}
func TestHealthReadiness(t *testing.T) {
	for _, offline := range []bool{false, true} {
		rec := request(api.NewHandler(&fakeStore{failure: offline}), "GET", "/health", "")
		want := 200
		if offline {
			want = 503
		}
		if rec.Code != want {
			t.Fatalf("status=%d want %d", rec.Code, want)
		}
		var result struct {
			Status string `json:"status"`
			DB     bool   `json:"db"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.DB == offline {
			t.Fatalf("wrong readiness: %+v", result)
		}
	}
}
func TestCreateListAndGet(t *testing.T) {
	h := api.NewHandler(&fakeStore{})
	if rec := request(h, "GET", "/api/notes", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatal(rec.Body.String())
	}
	rec := request(h, "POST", "/api/notes", `{"title":" first ","body":"hello"}`)
	if rec.Code != 201 || rec.Header().Get("Location") != "/api/notes/1" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = request(h, "GET", "/api/notes/1", "")
	var n api.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || n.Title != "first" || n.Body != "hello" || n.ID != 1 {
		t.Fatalf("get: %+v", n)
	}
	rec = request(h, "GET", "/api/notes", "")
	var notes []api.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &notes); err != nil || len(notes) != 1 {
		t.Fatalf("list: %s", rec.Body.String())
	}
}
func TestRejectInvalidInputs(t *testing.T) {
	for _, body := range []string{`{`, `{"title":" "}`, `{"title":"ok","unknown":true}`, `{"title":"ok"} {}`, `null`} {
		rec := request(api.NewHandler(&fakeStore{}), "POST", "/api/notes", body)
		if rec.Code != 400 {
			t.Fatalf("%s: status=%d", body, rec.Code)
		}
	}
	rec := request(api.NewHandler(&fakeStore{}), "POST", "/api/notes", `{"title":"ok","body":"`+strings.Repeat("x", 1<<20)+`"}`)
	if rec.Code != 413 {
		t.Fatalf("oversized: status=%d", rec.Code)
	}
	for _, id := range []string{"abc", "0", "-1"} {
		if rec := request(api.NewHandler(&fakeStore{}), "GET", "/api/notes/"+id, ""); rec.Code != 400 {
			t.Fatal(rec.Code)
		}
	}
}
func TestNotFoundDatabaseFailureAndMethod(t *testing.T) {
	h := api.NewHandler(&fakeStore{})
	if rec := request(h, "GET", "/api/notes/99", ""); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	if rec := request(h, "DELETE", "/api/notes", ""); rec.Code != 405 {
		t.Fatal(rec.Code)
	}
	h = api.NewHandler(&fakeStore{failure: true})
	for _, path := range []string{"/api/notes", "/api/notes/1"} {
		if rec := request(h, "GET", path, ""); rec.Code != 503 {
			t.Fatal(rec.Code)
		}
	}
	if rec := request(h, "POST", "/api/notes", `{"title":"ok"}`); rec.Code != 503 {
		t.Fatal(rec.Code)
	}
}
