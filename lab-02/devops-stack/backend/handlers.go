package main

import (
	"net/http"
	"notes/internal/api"
)

func newHandler(store api.Store) http.Handler { return api.NewHandler(store) }
