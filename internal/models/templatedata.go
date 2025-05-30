package models

import "github.com/malisalan/sideproject/internal/forms"

type TemplateData struct {
	StringMap       map[string]string
	IntMap          map[string]int
	FloatMap        map[string]float32
	Data            map[string]interface{}
	CSRFToken       string
	Flash           string
	Warning         string
	Error           string
	Form            *forms.Form
	IsAuthenticated int
	IsAdmin         bool
	Access_Level    int
	AllRooms        []Room
}
