package main

import (
	"fmt"
	"testing"

	"github.com/go-chi/chi"
	"github.com/malisalan/sideproject/internal/config"
)

func TestRoutes(t *testing.T) {
	var app config.AppConfig
	mux := routes(&app)
	switch v := mux.(type) {
	case *chi.Mux:
	default:
		t.Error(fmt.Sprintf("type is not *chi.Mux, but is %T", v))
	}
}
