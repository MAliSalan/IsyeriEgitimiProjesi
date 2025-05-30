package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/malisalan/sideproject/internal/models"
)

var theTests = []struct {
	name               string
	url                string
	method             string
	expectedStatusCode int
}{
	{"home", "/", "GET", http.StatusOK},
	{"about", "/about", "GET", http.StatusOK},
	{"contact", "/contact", "GET", http.StatusOK},
	{"generals-quarters", "/generals-quarters", "GET", http.StatusOK},
	{"majors-suite", "/majors-suite", "GET", http.StatusOK},
	{"search-availability", "/search-availability", "GET", http.StatusOK},
	{"login", "/user/login", "GET", http.StatusOK},
	{"register", "/user/register", "GET", http.StatusOK},
}

func TestHandlers(t *testing.T) {
	routes := getRoutes()
	testserver := httptest.NewTLSServer(routes)
	defer testserver.Close()
	for _, e := range theTests {
		resp, err := testserver.Client().Get(testserver.URL + e.url)
		if err != nil {
			t.Log(err)
			t.Fatal(err)
		}
		if resp.StatusCode != e.expectedStatusCode {
			t.Errorf("for %s expected %d but got %d", e.name, e.expectedStatusCode, resp.StatusCode)
		}
	}
}

func TestRepository_Reservation(t *testing.T) {

	reservations := models.Reservations{
		RoomID: 1,
		Room: models.Rooms{
			ID:       1,
			RoomName: "Generals Quarters",
		},
	}
	req, _ := http.NewRequest("GET", "/make-reservation", nil)
	ctx := getCTX(req)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	session.Put(ctx, "reservation", reservations)
	handler := http.HandlerFunc(Repo.Reservation)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/make-reservation", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusTemporaryRedirect, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/make-reservation", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	reservations.RoomID = 3
	session.Put(ctx, "reservation", reservations)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusTemporaryRedirect, rr.Code)
	}
}

func TestRepository_ReservationSummary(t *testing.T) {

	reservations := models.Reservations{
		RoomID:    1,
		StartDate: time.Now(),
		EndDate:   time.Now().AddDate(0, 0, 1),
		Room: models.Rooms{
			ID:       1,
			RoomName: "Generals Quarters",
		},
	}
	req, _ := http.NewRequest("GET", "/reservation-summary", nil)
	ctx := getCTX(req)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	session.Put(ctx, "reservation", reservations)
	handler := http.HandlerFunc(Repo.ReservationSummary)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/reservation-summary", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}
}

func TestRoomRoutes(t *testing.T) {
	req, _ := http.NewRequest("GET", "/general-rooms", nil)
	q := req.URL.Query()
	q.Add("id", "1")
	q.Add("s", "2025-01-01")
	q.Add("e", "2025-01-02")
	req.URL.RawQuery = q.Encode()

	ctx := getCTX(req)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Repo.GeneralRoom)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/major-rooms", nil)
	q = req.URL.Query()
	q.Add("id", "2")
	q.Add("s", "2025-01-01")
	q.Add("e", "2025-01-02")
	req.URL.RawQuery = q.Encode()

	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.MajorsRoom)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}
}

func TestRepository_AvailabilityJSON(t *testing.T) {

	postedData := url.Values{}
	postedData.Add("start", "2025-01-01")
	postedData.Add("end", "2025-01-02")
	postedData.Add("room_id", "1")

	req, _ := http.NewRequest("POST", "/search-availability-json", strings.NewReader(postedData.Encode()))
	ctx := getCTX(req)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Repo.AvailabilityJSON)
	handler.ServeHTTP(rr, req)

	var j jsonResponse
	err := json.Unmarshal(rr.Body.Bytes(), &j)
	if err != nil {
		t.Error("JSON response could not be parsed")
	}
}

func TestAuthHandlers(t *testing.T) {
	req, _ := http.NewRequest("GET", "/user/login", nil)
	ctx := getCTX(req)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Repo.Login)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData := url.Values{}
	postedData.Add("email", "invalid-email")
	postedData.Add("password", "12")

	req, _ = http.NewRequest("POST", "/user/login", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostLogin)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData = url.Values{}
	postedData.Add("email", "test@gmail.com")
	postedData.Add("password", "password123")

	req, _ = http.NewRequest("POST", "/user/login", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostLogin)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/user/logout", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.Logout)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/user/register", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.Register)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData = url.Values{}
	postedData.Add("first_name", "Test")
	postedData.Add("last_name", "User")
	postedData.Add("email", "test@testdenem.com")
	postedData.Add("password", "password123")

	req, _ = http.NewRequest("POST", "/user/register", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostRegister)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}
}

func TestProfileHandlers(t *testing.T) {
	req, _ := http.NewRequest("GET", "/user/profile", nil)
	ctx := getCTX(req)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Repo.Profile)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/user/profile", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.Profile)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData := url.Values{}
	postedData.Add("first_name", "Updated")
	postedData.Add("last_name", "User")
	postedData.Add("phone", "555-123-4567")

	req, _ = http.NewRequest("POST", "/user/profile/update", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostProfileUpdate)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData = url.Values{}
	postedData.Add("current_password", "password123")
	postedData.Add("new_password", "newpassword123")
	postedData.Add("confirm_password", "newpassword123")

	req, _ = http.NewRequest("POST", "/user/password/update", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostPasswordUpdate)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusOK, rr.Code)
	}

	postedData = url.Values{}
	postedData.Add("card_name", "Test User")
	postedData.Add("card_number", "1111111111111111")
	postedData.Add("expiry_date", "12/25")
	postedData.Add("cvv", "123")

	req, _ = http.NewRequest("POST", "/user/payment/add", strings.NewReader(postedData.Encode()))
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.PostPaymentAdd)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d veya %d'dir ancak gelen durum kodu %d'dir",
			http.StatusOK, http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/user/payment/delete/1", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.DeletePaymentMethod)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}

	req, _ = http.NewRequest("GET", "/user/reservation/cancel/1", nil)
	ctx = getCTX(req)
	req = req.WithContext(ctx)
	session.Put(ctx, "user_id", 1)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(Repo.CancelReservation)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("beklenen durum kodu %d'dir ancak gelen durum kodu %d'dir", http.StatusSeeOther, rr.Code)
	}
}

func getCTX(req *http.Request) context.Context {
	ctx, err := session.Load(req.Context(), req.Header.Get("X-Session"))
	if err != nil {
		log.Println(err)
	}
	return ctx
}
