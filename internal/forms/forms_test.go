package forms

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestForm_Valid(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := New(r.PostForm)

	isValid := form.Valid()
	if !isValid {
		t.Error("got invalid when should have been valid")
	}
}

func TestForm_Required(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := New(r.PostForm)

	form.Required("a", "b", "c")
	if form.Valid() {
		t.Error("form shows valid when required fields missing")
	}

	postedData := url.Values{}
	postedData.Add("a", "a")
	postedData.Add("b", "a")
	postedData.Add("c", "a")

	r, _ = http.NewRequest("POST", "/whatever", nil)

	r.PostForm = postedData
	form = New(r.PostForm)
	form.Required("a", "b", "c")
	if !form.Valid() {
		t.Error("shows does not have required fields when it does")
	}
}

func TestForm_Has(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := New(r.PostForm)

	has := form.Has("whatever", r)
	if has {
		t.Error("form oluşturuken hata")
	}

	postedData := url.Values{}
	postedData.Add("a", "a")
	form = New(postedData)
	has = form.Has("a", r)
	if !has {
		t.Error("form oluşturuken hata")
	}

}
func TestForm_MinLength(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := New(r.PostForm)

	form.MinLength("x", 10, r)
	if form.Valid() {
		t.Error("minimum uzunluk 10 olmasına rağmen hiçbir şey girilmediğinden hata verdi")
	}
	postedvalues := url.Values{}
	postedvalues.Add("deneme", "some_value")
	form = New(postedvalues)
	form.MinLength("deneme", 100, r)
	if form.Valid() {
		t.Error("minimum uzunluk 100 olmasına rağmen 10 karakterden az girildiğinden hata verdi")
	}
	postedvalues = url.Values{}
	postedvalues.Add("deneme2", "abc")
	form = New(postedvalues)
	form.MinLength("deneme2", 2, r)
	if !form.Valid() {
		t.Error("minimum uzunluk 2 olmasına rağmen 2 karakterden az girildiğinden hata verdi")
	}
}

func TestForm_IsEmail(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := New(r.PostForm)
	form.IsEmail("email")
	if form.Valid() {
		t.Error("geçersiz e-posta adresi girildiğinde hata verdi")
	}
	postedvalues := url.Values{}
	postedvalues.Add("email", "abc")
	form = New(postedvalues)
	form.IsEmail("email")
	if form.Valid() {
		t.Error("geçersiz e-posta adresi girildiğinde hata verdi")
	}
	postedvalues = url.Values{}
	postedvalues.Add("email", "asdasdasda@gmail.com")
	form = New(postedvalues)
	form.IsEmail("email")
	if !form.Valid() {
		t.Error("geçerli e-posta adresi girilmediğinden hata verdi")
	}
	postedvalues = url.Values{}
	postedvalues.Add("email", "asdasdasda@qweqweq.com")
	form = New(postedvalues)
	form.IsEmail("email")
	if form.Valid() {
		t.Error("geçersiz e-posta adresi girildiğinde hata verdi")
	}

}

func TestErrors_Get(t *testing.T) {
	var errs errors = errors{
		"empty": {},
		"dummy": {"bir hata mesajı"},
	}
	emptyErr := errs.Get("empty")
	if emptyErr != "" {
		t.Errorf("boş hata mesajı bekleniyordu, '%s' alındı", emptyErr)
	}

	if errs.Get("dummy") != "bir hata mesajı" {
		t.Errorf("dummy hata mesajı bekleniyordu")
	}
}
