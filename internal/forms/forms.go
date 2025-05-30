package forms

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/asaskevich/govalidator"
)

type Form struct {
	url.Values
	Errors errors
}

func (f *Form) Valid() bool {
	return len(f.Errors) == 0
}

func New(data url.Values) *Form {
	return &Form{
		data,
		errors(map[string][]string{}),
	}
}

func (f *Form) Required(fields ...string) {
	for _, field := range fields {
		value := f.Get(field)
		if strings.TrimSpace(value) == "" {
			f.Errors.Add(field, "Bu alan boş bırakılamaz")
		}
	}
}

func (f *Form) Has(field string, r *http.Request) bool {
	x := f.Get(field)
	if x == "" {
		f.Errors.Add(field, "Bu alan boş bırakılamaz")
		return false
	}
	return true
}

func (f *Form) MinLength(field string, length int, r *http.Request) bool {
	x := f.Get(field)
	if len(x) < length {
		f.Errors.Add(field, fmt.Sprintf("Bu alan en az %d karakter olmalıdır", length))
		return false
	}
	return true
}

func (f *Form) IsEmail(field string) {
	email := f.Get(field)
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		f.Errors.Add(field, "Lütfen e-posta adresinizi tam girin")
		return
	}
	if !govalidator.IsEmail(email) || (!strings.HasSuffix(email, "@gmail.com") && !strings.HasSuffix(email, "@outlook.com") && !strings.HasSuffix(email, "@testdenem.com") && !strings.HasSuffix(email, "@example.com")) {
		f.Errors.Add(field, "Geçerli bir e-posta adresi giriniz")
	}
}
