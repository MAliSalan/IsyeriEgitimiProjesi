package helpers

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/malisalan/sideproject/internal/config"
)

var app *config.AppConfig

func NewHelpers(a *config.AppConfig) {
	app = a
}

func ClientErroe(w http.ResponseWriter, status int) {
	app.InfoLog.Println("Sunucu tarafında bir hata oluştu. Hata kodu:", status)
	http.Error(w, http.StatusText(status), status)
}

func ServerError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.ErrorLog.Println(trace)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func IsAuthenticated(r *http.Request) bool {
	exists := app.Session.Exists(r.Context(), "user_id")
	return exists
}

func IsAdmin(r *http.Request) bool {
	if !IsAuthenticated(r) {
		return false
	}

	userID := app.Session.GetInt(r.Context(), "user_id")
	accessLevel := app.Session.GetInt(r.Context(), "access_level")

	return userID > 0 && accessLevel >= 3
}
