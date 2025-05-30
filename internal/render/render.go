package render

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/justinas/nosurf"
	"github.com/malisalan/sideproject/internal/config"
	"github.com/malisalan/sideproject/internal/helpers"
	"github.com/malisalan/sideproject/internal/models"
	"github.com/malisalan/sideproject/internal/repository"
)

var functions = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
	"subtract": func(a, b int) int {
		return a - b
	},
	"iterate": func(start, end int) []int {
		var result []int
		for i := start; i <= end; i++ {
			result = append(result, i)
		}
		return result
	},
}

var app *config.AppConfig
var pathToTemplates = "./templates"
var Repo repository.DatabaseRepo

func NewRenderer(a *config.AppConfig) {
	app = a
}

func NewRepo(r repository.DatabaseRepo) {
	Repo = r
}

func AddDefaultData(td *models.TemplateData, r *http.Request) *models.TemplateData {
	td.Flash = app.Session.PopString(r.Context(), "flash")
	td.Warning = app.Session.PopString(r.Context(), "warning")
	td.Error = app.Session.PopString(r.Context(), "error")
	td.CSRFToken = nosurf.Token(r)

	if helpers.IsAuthenticated(r) {
		td.IsAuthenticated = 1
	} else {
		td.IsAuthenticated = 0
	}

	td.IsAdmin = helpers.IsAdmin(r)

	td.Access_Level = app.Session.GetInt(r.Context(), "access_level")

	rooms, err := Repo.GetAllRooms()
	if err != nil {
		app.ErrorLog.Printf("Odalar alınırken hata: %v", err)
	} else {
		td.AllRooms = rooms
	}

	return td
}

func Template(w http.ResponseWriter, r *http.Request, tmpl string, td *models.TemplateData) {
	var tc map[string]*template.Template

	if app.UseCache {
		tc = app.TemplateCache
	} else {
		var err error
		tc, err = CreateTemplateCache()
		if err != nil {
			app.ErrorLog.Printf("Şablon önbelleği oluşturulurken hata: %v", err)
			http.Error(w, "Sayfa şu anda görüntülenemiyor", http.StatusInternalServerError)
			return
		}
	}

	t, ok := tc[tmpl]
	if !ok {
		app.ErrorLog.Printf("Şablon bulunamadı: %s", tmpl)
		http.Error(w, "Sayfa şu anda görüntülenemiyor", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)

	td = AddDefaultData(td, r)

	if td.Data != nil && len(td.Data) > 0 {
		app.InfoLog.Printf("Şablon verilerinin anahtarları: %v", getMapKeys(td.Data))
	}

	err := t.Execute(buf, td)
	if err != nil {
		app.ErrorLog.Printf("Şablon işlenirken hata: %v", err)
		http.Error(w, "Sayfa şu anda görüntülenemiyor", http.StatusInternalServerError)
		return
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		app.ErrorLog.Printf("Şablon tarayıcıya yazılırken hata: %v", err)
		http.Error(w, "Sayfa şu anda görüntülenemiyor", http.StatusInternalServerError)
		return
	}
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func CreateTemplateCache() (map[string]*template.Template, error) {

	myCache := map[string]*template.Template{}

	pages, err := filepath.Glob(fmt.Sprintf("%s/*.page.tmpl", pathToTemplates))
	if err != nil {
		return myCache, err
	}

	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(functions).ParseFiles(page)
		if err != nil {
			return myCache, err
		}

		matches, err := filepath.Glob(fmt.Sprintf("%s/*.layout.tmpl", pathToTemplates))
		if err != nil {
			return myCache, err
		}

		if len(matches) > 0 {
			ts, err = ts.ParseGlob(fmt.Sprintf("%s/*.layout.tmpl", pathToTemplates))
			if err != nil {
				return myCache, err
			}
		}

		myCache[name] = ts
	}

	return myCache, nil
}
