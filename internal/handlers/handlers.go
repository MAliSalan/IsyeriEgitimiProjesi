package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/go-chi/chi"
	"github.com/malisalan/sideproject/internal/config"
	"github.com/malisalan/sideproject/internal/driver"
	"github.com/malisalan/sideproject/internal/forms"
	"github.com/malisalan/sideproject/internal/models"
	"github.com/malisalan/sideproject/internal/render"
	"github.com/malisalan/sideproject/internal/repository"
	"github.com/malisalan/sideproject/internal/repository/dbrepo"
	"golang.org/x/crypto/bcrypt"
)

var Repo *Repository

type Repository struct {
	App *config.AppConfig
	DB  repository.DatabaseRepo
}

func init() {
	gob.Register(models.Reservations{})
}
func NewRepo(a *config.AppConfig, db *driver.DB) *Repository {
	return &Repository{
		App: a,
		DB:  dbrepo.NewMySQLRepo(db.SQL, a),
	}
}
func NewTestRepo(a *config.AppConfig) *Repository {
	return &Repository{
		App: a,
		DB:  dbrepo.NewTestingRepo(a),
	}
}

func NewHandlers(r *Repository) {
	Repo = r
}

func (m *Repository) Home(w http.ResponseWriter, r *http.Request) {

	render.Template(w, r, "home.page.tmpl", &models.TemplateData{})
}

func (m *Repository) About(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "about.page.tmpl", &models.TemplateData{})
}

func (m *Repository) Reservation(w http.ResponseWriter, r *http.Request) {
	res, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservations)
	if !ok {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	rooms, err := m.DB.GetRoomByID(res.RoomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Oda bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	res.Room.RoomName = rooms.RoomName

	roomInfo, err := m.DB.GetRoomInfoByRoomID(res.RoomID)
	if err != nil {
		roomInfo = models.RoomInfo{
			RoomMaxCap:     2,
			RoomDailyPrice: 500,
		}
	}

	totalDays := int(res.EndDate.Sub(res.StartDate).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}
	totalAmount := totalDays * roomInfo.RoomDailyPrice

	var paymentMethods []models.PaymentMethod
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID > 0 {
		user, err := m.DB.GetUserByID(userID)
		if err == nil {
			if res.FirstName == "" && res.LastName == "" && res.Email == "" {
				res.FirstName = user.Firstname
				res.LastName = user.LastName
				res.Email = user.Email
				res.Phone = user.Phone
			}
		}

		paymentMethods, err = m.DB.GetPaymentMethodsByUserID(userID)
		if err != nil {
			m.App.ErrorLog.Printf("Kullanıcının ödeme yöntemleri alınamadı: %v", err)
			paymentMethods = []models.PaymentMethod{}
		}
	}

	m.App.Session.Put(r.Context(), "reservation", res)

	sd := res.StartDate.Format("2006-01-02")
	ed := res.EndDate.Format("2006-01-02")

	stringMap := make(map[string]string)
	stringMap["start_date"] = sd
	stringMap["end_date"] = ed

	data := make(map[string]interface{})
	data["reservation"] = res
	data["room_info"] = roomInfo
	data["total_days"] = totalDays
	data["total_amount"] = totalAmount
	data["payment_methods"] = paymentMethods

	render.Template(w, r, "make-reservation.page.tmpl", &models.TemplateData{
		Form:      forms.New(nil),
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) PostReservation(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	reservations, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservations)
	if !ok {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	reservations.FirstName = r.Form.Get("first_name")
	reservations.LastName = r.Form.Get("last_name")
	reservations.Email = r.Form.Get("email")
	reservations.Phone = r.Form.Get("phone")

	totalAmountStr := r.Form.Get("total_amount")
	paymentMethod := r.Form.Get("payment_method")
	savedCardID := r.Form.Get("saved_card")

	var totalAmount int
	if totalAmountStr != "" {
		totalAmount, _ = strconv.Atoi(totalAmountStr)
	}

	var selectedPaymentMethod models.PaymentMethod
	var usesSavedCard bool
	if paymentMethod == "card" && savedCardID != "" && savedCardID != "new" {
		cardID, err := strconv.Atoi(savedCardID)
		if err == nil {
			selectedPaymentMethod, err = m.DB.GetPaymentMethodByID(cardID)
			if err == nil {
				userID := m.App.Session.GetInt(r.Context(), "user_id")
				isOwner, err := m.DB.IsPaymentMethodOwner(cardID, userID)
				if err == nil && isOwner {
					usesSavedCard = true
				}
			}
		}
	}

	form := forms.New(r.PostForm)
	form.Required("first_name", "last_name", "email")
	form.MinLength("first_name", 3, r)
	form.MinLength("last_name", 3, r)
	form.IsEmail("email")

	if !form.Valid() {
		roomInfo, err := m.DB.GetRoomInfoByRoomID(reservations.RoomID)
		if err != nil {
			roomInfo = models.RoomInfo{
				RoomMaxCap:     2,
				RoomDailyPrice: 500,
			}
		}
		totalDays := int(reservations.EndDate.Sub(reservations.StartDate).Hours() / 24)
		if totalDays <= 0 {
			totalDays = 1
		}
		if totalAmount == 0 {
			totalAmount = totalDays * roomInfo.RoomDailyPrice
		}

		sd := reservations.StartDate.Format("2006-01-02")
		ed := reservations.EndDate.Format("2006-01-02")
		stringMap := make(map[string]string)
		stringMap["start_date"] = sd
		stringMap["end_date"] = ed

		data := make(map[string]interface{})
		data["reservation"] = reservations
		data["room_info"] = roomInfo
		data["total_days"] = totalDays
		data["total_amount"] = totalAmount

		render.Template(w, r, "make-reservation.page.tmpl", &models.TemplateData{
			Form:      form,
			Data:      data,
			StringMap: stringMap,
		})
		return
	}

	reservations.TotalAmount = totalAmount
	reservations.PaymentMethod = paymentMethod

	var paymentStatus string
	var paymentDate *time.Time
	now := time.Now()

	switch paymentMethod {
	case "card":
		if usesSavedCard {
			m.App.InfoLog.Printf("Kayıtlı kart kullanılıyor: %s **** %s", selectedPaymentMethod.CardType, selectedPaymentMethod.LastFour)
			paymentStatus = "paid"
			paymentDate = &now
			reservations.PaymentStatus = "paid"
		} else {
			cardNumber := r.Form.Get("card_number")
			cardName := r.Form.Get("card_name")
			m.App.InfoLog.Printf("Yeni kart kullanılıyor: %s için %s", cardName, cardNumber)
			paymentStatus = "paid"
			paymentDate = &now
			reservations.PaymentStatus = "paid"
		}
	case "balance":
		paymentStatus = "paid"
		paymentDate = &now
		reservations.PaymentStatus = "paid"
	case "later":
		paymentStatus = "pending"
		paymentDate = nil
		reservations.PaymentStatus = "pending"
	default:
		paymentStatus = "pending"
		paymentDate = nil
		reservations.PaymentStatus = "pending"
	}

	NewReservationID, err := m.DB.InsertReservation(reservations)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon kaydedilemedi: %v", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon kaydedilirken bir hata oluştu. Lütfen tekrar deneyin.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	reservations.ID = NewReservationID

	payStatus := models.ReservationPayStatus{
		ReservationID: NewReservationID,
		TotalAmount:   totalAmount,
		PaymentStatus: paymentStatus,
		PaymentMethod: paymentMethod,
		PaymentDate:   paymentDate,
	}

	_, err = m.DB.InsertReservationPayStatus(payStatus)
	if err != nil {
		m.App.ErrorLog.Printf("Ödeme durumu kaydedilemedi: %v", err)
	}

	m.App.Session.Put(r.Context(), "reservation", reservations)

	restrictions := models.RoomRestrictions{
		StartDate:     reservations.StartDate,
		EndDate:       reservations.EndDate,
		RoomID:        reservations.RoomID,
		ReservationID: NewReservationID,
		RestrictionID: 1,
	}
	err = m.DB.InsertRoomRestrictions(restrictions)
	if err != nil {
		m.App.ErrorLog.Printf("Oda kısıtlaması kaydedilemedi: %v", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon tamamlanırken bir hata oluştu. Lütfen müşteri hizmetleriyle iletişime geçin.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(reservations.RoomID)
	if err != nil {
		roomInfo = models.RoomInfo{
			RoomMaxCap:     2,
			RoomDailyPrice: 500,
		}
	}

	totalDays := int(reservations.EndDate.Sub(reservations.StartDate).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}

	type MailTemplateData struct {
		FirstName     string
		LastName      string
		RoomName      string
		StartDate     string
		EndDate       string
		Phone         string
		Email         string
		Year          int
		Capacity      int
		Nights        int
		DailyPrice    int
		TotalPrice    int
		PaymentStatus string
		PaymentMethod string
		PaymentDate   string
	}

	mailData := MailTemplateData{
		FirstName:     reservations.FirstName,
		LastName:      reservations.LastName,
		RoomName:      reservations.Room.RoomName,
		StartDate:     reservations.StartDate.Format("2006-01-02"),
		EndDate:       reservations.EndDate.Format("2006-01-02"),
		Phone:         reservations.Phone,
		Email:         reservations.Email,
		Year:          time.Now().Year(),
		Capacity:      roomInfo.RoomMaxCap,
		Nights:        totalDays,
		DailyPrice:    roomInfo.RoomDailyPrice,
		TotalPrice:    totalAmount,
		PaymentStatus: paymentStatus,
		PaymentMethod: paymentMethod,
		PaymentDate: func() string {
			if paymentDate != nil {
				return paymentDate.Format("2006-01-02")
			}
			return "Henüz ödenmedi"
		}(),
	}

	var tpl bytes.Buffer
	tmpl, err := template.ParseFiles("./templates/reservation-pending.mail.tmpl")
	if err != nil {
		m.App.ErrorLog.Printf("Mail şablonu yüklenemedi: %v", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon onay maili gönderilemedi")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	err = tmpl.Execute(&tpl, mailData)
	if err != nil {
		m.App.ErrorLog.Printf("Mail şablonu işlenemedi: %v", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon onay maili gönderilemedi")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	msg := models.MailData{
		To:      reservations.Email,
		From:    "muhammed@here.com",
		Subject: "Rezervasyon Durumu",
		Content: tpl.String(),
	}
	m.App.MailChan <- msg

	m.App.Session.Put(r.Context(), "reservation", reservations)
	http.Redirect(w, r, "/reservation-summary", http.StatusSeeOther)
}

func (m *Repository) Generals(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "generals.page.tmpl", &models.TemplateData{})
}
func (m *Repository) Majors(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "majors.page.tmpl", &models.TemplateData{})
}
func (m *Repository) Availability(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "search-availability.page.tmpl", &models.TemplateData{})
}
func (m *Repository) PostAvailability(w http.ResponseWriter, r *http.Request) {
	start := r.Form.Get("start")
	end := r.Form.Get("end")

	layout := "2006-01-02"
	startDate, err := time.Parse(layout, start)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	endDate, err := time.Parse(layout, end)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	rooms, err := m.DB.SearchAvailabilityForAllRooms(startDate, endDate)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	if len(rooms) == 0 {
		m.App.Session.Put(r.Context(), "error", "Bu tarihlerde uygun bir oda bulunamadı")
		http.Redirect(w, r, "/search-availability", http.StatusSeeOther)
		return
	}

	type RoomWithInfo struct {
		Room     models.Rooms
		RoomInfo models.RoomInfo
	}

	var roomsWithInfo []RoomWithInfo
	for _, room := range rooms {
		roomInfo, err := m.DB.GetRoomInfoByRoomID(room.ID)
		if err != nil {
			roomInfo = models.RoomInfo{
				RoomMaxCap:     1,
				RoomDailyPrice: 0,
			}
		}
		roomsWithInfo = append(roomsWithInfo, RoomWithInfo{
			Room:     room,
			RoomInfo: roomInfo,
		})
	}

	data := make(map[string]interface{})
	data["rooms"] = rooms
	data["roomsWithInfo"] = roomsWithInfo
	res := models.Reservations{
		StartDate: startDate,
		EndDate:   endDate,
	}
	m.App.Session.Put(r.Context(), "reservation", res)
	render.Template(w, r, "choose-room.page.tmpl", &models.TemplateData{
		Data: data,
	})
}

type jsonResponse struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	RoomID    string `json:"room_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (m *Repository) AvailabilityJSON(w http.ResponseWriter, r *http.Request) {

	sd := r.Form.Get("start")
	ed := r.Form.Get("end")
	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)
	roomID, _ := strconv.Atoi(r.Form.Get("room_id"))
	avaible, _ := m.DB.SearchAvailabilityByDatesByRoomID(startDate, endDate, roomID)
	resp := jsonResponse{
		OK:        avaible,
		Message:   "",
		StartDate: sd,
		EndDate:   ed,
		RoomID:    strconv.Itoa(roomID),
	}
	out, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	log.Println(string(out))
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (m *Repository) Contact(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "contact.page.tmpl", &models.TemplateData{
		Form: forms.New(nil),
	})
}

func (m *Repository) ReservationSummary(w http.ResponseWriter, r *http.Request) {
	reservation, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservations)
	if !ok {
		m.App.ErrorLog.Println("Rezervasyon bilgileri alınamadı")
		log.Println("Rezervasyonun alınması başarısız oldu lütfen tekrar deneyiniz")
		m.App.Session.Put(r.Context(), "error", "Rezervasyonun alınması başarısız oldu lütfen tekrar deneyiniz")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(reservation.RoomID)
	if err != nil {
		m.App.ErrorLog.Printf("Oda bilgileri alınamadı: %v", err)
		roomInfo = models.RoomInfo{
			RoomMaxCap:     2,
			RoomDailyPrice: 500,
		}
	}

	totalDays := int(reservation.EndDate.Sub(reservation.StartDate).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}
	totalAmount := totalDays * roomInfo.RoomDailyPrice

	var paymentInfo models.ReservationPayStatus
	if reservation.ID > 0 {
		paymentInfo, err = m.DB.GetReservationPayStatusByReservationID(reservation.ID)
		if err != nil {
			paymentInfo = models.ReservationPayStatus{
				TotalAmount:   totalAmount,
				PaymentStatus: reservation.PaymentStatus,
				PaymentMethod: reservation.PaymentMethod,
			}
		}
	} else {
		paymentInfo = models.ReservationPayStatus{
			TotalAmount:   totalAmount,
			PaymentStatus: reservation.PaymentStatus,
			PaymentMethod: reservation.PaymentMethod,
		}
	}

	m.App.Session.Remove(r.Context(), "reservation")
	data := make(map[string]interface{})
	data["reservation"] = reservation
	data["room_info"] = roomInfo
	data["total_days"] = totalDays
	data["total_amount"] = totalAmount
	data["payment_info"] = paymentInfo

	sd := reservation.StartDate.Format("2006-01-02")
	ed := reservation.EndDate.Format("2006-01-02")
	displayStartDate := reservation.StartDate.Format("02.01.2006")
	displayEndDate := reservation.EndDate.Format("02.01.2006")

	stringMap := make(map[string]string)
	stringMap["start_date"] = sd
	stringMap["end_date"] = ed
	stringMap["display_start_date"] = displayStartDate
	stringMap["display_end_date"] = displayEndDate

	render.Template(w, r, "reservation-summary.page.tmpl", &models.TemplateData{
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) ChooseRoom(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	res, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservations)
	if !ok {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	res.RoomID = roomID
	m.App.Session.Put(r.Context(), "reservation", res)
	http.Redirect(w, r, "/make-reservation", http.StatusSeeOther)

}
func (m *Repository) GeneralRoom(w http.ResponseWriter, r *http.Request) {
	roomID, _ := strconv.Atoi(r.URL.Query().Get("id"))
	sd := r.URL.Query().Get("s")
	ed := r.URL.Query().Get("e")
	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)

	var res models.Reservations
	rooms, err := m.DB.GetRoomByID(roomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	res.Room.RoomName = rooms.RoomName

	res.RoomID = roomID
	res.StartDate = startDate
	res.EndDate = endDate
	m.App.Session.Put(r.Context(), "reservation", res)

	http.Redirect(w, r, "/make-reservation", http.StatusSeeOther)
}
func (m *Repository) MajorsRoom(w http.ResponseWriter, r *http.Request) {
	roomID, _ := strconv.Atoi(r.URL.Query().Get("id"))
	sd := r.URL.Query().Get("s")
	ed := r.URL.Query().Get("e")
	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)

	var res models.Reservations
	rooms, err := m.DB.GetRoomByID(roomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	res.Room.RoomName = rooms.RoomName

	res.RoomID = roomID
	res.StartDate = startDate
	res.EndDate = endDate
	m.App.Session.Put(r.Context(), "reservation", res)

	http.Redirect(w, r, "/make-reservation", http.StatusSeeOther)
}

func (m *Repository) Login(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "login.page.tmpl", &models.TemplateData{
		Form: forms.New(nil),
	})
}

func (m *Repository) PostLogin(w http.ResponseWriter, r *http.Request) {
	_ = m.App.Session.RenewToken(r.Context())
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	email := r.Form.Get("email")
	password := r.Form.Get("password")

	form := forms.New(r.PostForm)
	form.Required("email", "password")
	form.IsEmail("email")
	form.MinLength("password", 3, r)
	if !form.Valid() {
		render.Template(w, r, "login.page.tmpl", &models.TemplateData{
			Form: form,
		})
		return
	}

	id, _, err := m.DB.Authenticate(email, password)
	if err != nil {
		log.Println(err)
		m.App.Session.Put(r.Context(), "error", "Giriş bilgileri alınamadı")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	user, err := m.DB.GetUserByID(id)
	if err != nil {
		log.Println(err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	if user.AccActStatus != "confirmed" {
		m.App.Session.Put(r.Context(), "error", "Hesabınız henüz onaylanmamış. Lütfen e-posta adresinizi doğrulayın.")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	m.App.Session.Put(r.Context(), "user_id", id)
	m.App.Session.Put(r.Context(), "access_level", user.Accsesslevel)
	m.App.Session.Put(r.Context(), "flash", "Başarıyla giriş yaptınız")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (m *Repository) Logout(w http.ResponseWriter, r *http.Request) {
	m.App.Session.Remove(r.Context(), "user_id")
	m.App.Session.Remove(r.Context(), "access_level")
	m.App.Session.Put(r.Context(), "flash", "Başarıyla çıkış yaptınız")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (m *Repository) Register(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "register.page.tmpl", &models.TemplateData{
		Form: forms.New(nil),
	})
}
func generateToken(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (m *Repository) PostRegister(w http.ResponseWriter, r *http.Request) {
	_ = m.App.Session.RenewToken(r.Context())
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/user/register", http.StatusSeeOther)
		return
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")
	firstName := r.Form.Get("first_name")
	lastName := r.Form.Get("last_name")

	form := forms.New(r.PostForm)
	form.Required("email", "password", "first_name", "last_name")
	form.IsEmail("email")
	form.MinLength("password", 8, r)
	form.MinLength("first_name", 2, r)
	form.MinLength("last_name", 2, r)

	if !form.Valid() {
		render.Template(w, r, "register.page.tmpl", &models.TemplateData{
			Form: form,
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Şifre hashleme hatası:", err)
		m.App.Session.Put(r.Context(), "error", "Kayıt işlemi sırasında bir hata oluştu")
		http.Redirect(w, r, "/user/register", http.StatusSeeOther)
		return
	}

	token, err := generateToken(16)
	if err != nil {
		log.Println("Token üretilemedi:", err)
		m.App.Session.Put(r.Context(), "error", "Kayıt işlemi sırasında bir hata oluştu (token)")
		http.Redirect(w, r, "/user/register", http.StatusSeeOther)
		return
	}

	user := models.Users{
		Firstname:       firstName,
		LastName:        lastName,
		Email:           email,
		Password:        string(hashedPassword),
		Accsesslevel:    1,
		AccActStatus:    "pending",
		ActivationToken: token,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err = m.DB.InsertUser(user)
	if err != nil {
		log.Println("Kullanıcı kayıt hatası:", err)
		m.App.Session.Put(r.Context(), "error", "Kayıt işlemi başarısız oldu. Email adresi zaten kullanımda olabilir.")
		http.Redirect(w, r, "/user/register", http.StatusSeeOther)
		return
	}

	activationLink := fmt.Sprintf("http://localhost:8080/verifyaccount?act_token=%s", token)
	var mailContent string
	var tpl bytes.Buffer
	tmpl, err := template.ParseFiles("./templates/activation.mail.tmpl")
	if err != nil {
		log.Println("Mail şablonu yüklenemedi:", err)
		mailContent = fmt.Sprintf("<h2>Hesabınızı doğrulamak için tıklayın:</h2><a href=\"%s\">Hesabımı Doğrula</a>", activationLink)
	} else {
		tmpl.ExecuteTemplate(&tpl, "activation_mail", map[string]interface{}{
			"FirstName":      firstName,
			"ActivationLink": activationLink,
		})
		mailContent = tpl.String()
	}
	msg := models.MailData{
		To:      email,
		From:    "admin@admin.com",
		Subject: "Hesap Aktivasyon",
		Content: mailContent,
	}
	m.App.MailChan <- msg

	m.App.Session.Put(r.Context(), "flash", "Başarıyla kayıt oldunuz. Lütfen e-posta adresinizi doğrulayın.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (m *Repository) VerifyAccount(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("act_token")
	data := make(map[string]interface{})
	if token == "" {
		data["Success"] = false
		data["Message"] = "Token eksik veya hatalı."
		render.Template(w, r, "activation-result.page.tmpl", &models.TemplateData{Data: data})
		return
	}
	ok, err := m.DB.ActivateUserByToken(token)
	if err != nil {
		data["Success"] = false
		data["Message"] = "Sunucu hatası oluştu."
		render.Template(w, r, "activation-result.page.tmpl", &models.TemplateData{Data: data})
		return
	}
	if !ok {
		data["Success"] = false
		data["Message"] = "Geçersiz veya kullanılmış token."
		render.Template(w, r, "activation-result.page.tmpl", &models.TemplateData{Data: data})
		return
	}
	data["Success"] = true
	render.Template(w, r, "activation-result.page.tmpl", &models.TemplateData{Data: data})
}

func (m *Repository) Profile(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")

	m.App.InfoLog.Printf("Profil sayfası için oturum açmış kullanıcı ID: %d\n", userID)

	if userID == 0 {
		m.App.Session.Put(r.Context(), "error", "Oturum açmanız gerekiyor")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	m.App.InfoLog.Printf("Kullanıcı bulundu: %s %s\n", user.Firstname, user.LastName)

	data := make(map[string]interface{})
	data["User"] = user

	render.Template(w, r, "profile.page.tmpl", &models.TemplateData{
		Data: data,
		Form: forms.New(nil),
	})
}

func (m *Repository) UserReservations(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")

	if userID == 0 {
		m.App.Session.Put(r.Context(), "error", "Oturum açmanız gerekiyor")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	reservations, err := m.DB.GetReservationsByUserID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon bilgileri alınamadı: %v\n", err)
		reservations = []models.Reservations{}
	}

	reservationPayments := make(map[int]models.ReservationPayStatus)
	for _, res := range reservations {
		paymentInfo, err := m.DB.GetReservationPayStatusByReservationID(res.ID)
		if err != nil {
			paymentInfo = models.ReservationPayStatus{
				ReservationID: res.ID,
				TotalAmount:   0,
				PaymentStatus: "unknown",
				PaymentMethod: "unknown",
			}
		}
		reservationPayments[res.ID] = paymentInfo
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Reservations"] = reservations
	data["ReservationPayments"] = reservationPayments

	stringMap := make(map[string]string)
	for _, res := range reservations {
		startDateStr := res.StartDate.Format("02.01.2006")
		endDateStr := res.EndDate.Format("02.01.2006")
		stringMap[fmt.Sprintf("start_date_%d", res.ID)] = startDateStr
		stringMap[fmt.Sprintf("end_date_%d", res.ID)] = endDateStr
	}

	render.Template(w, r, "reservations.page.tmpl", &models.TemplateData{
		Data:      data,
		Form:      forms.New(nil),
		StringMap: stringMap,
	})
}

func (m *Repository) UserPayments(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")

	if userID == 0 {
		m.App.Session.Put(r.Context(), "error", "Oturum açmanız gerekiyor")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	paymentMethods, err := m.DB.GetPaymentMethodsByUserID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Ödeme yöntemi bilgileri alınamadı: %v\n", err)
		paymentMethods = []models.PaymentMethod{}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["PaymentMethods"] = paymentMethods

	render.Template(w, r, "payments.page.tmpl", &models.TemplateData{
		Data: data,
		Form: forms.New(nil),
	})
}

func (m *Repository) UserPassword(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")

	if userID == 0 {
		m.App.Session.Put(r.Context(), "error", "Oturum açmanız gerekiyor")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := make(map[string]interface{})
	data["User"] = user

	render.Template(w, r, "password.page.tmpl", &models.TemplateData{
		Data: data,
		Form: forms.New(nil),
	})
}

func (m *Repository) PostProfileUpdate(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Form işlenirken bir hata oluştu"
		stringMap["active_tab"] = ""

		data := make(map[string]interface{})
		userID := m.App.Session.GetInt(r.Context(), "user_id")
		user, _ := m.DB.GetUserByID(userID)
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "profile.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      forms.New(nil),
		})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Kullanıcı bilgileri alınamadı"
		stringMap["active_tab"] = ""

		data := make(map[string]interface{})
		render.Template(w, r, "profile.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      forms.New(nil),
		})
		return
	}

	firstName := r.Form.Get("first_name")
	lastName := r.Form.Get("last_name")
	phone := r.Form.Get("phone")

	form := forms.New(r.PostForm)
	form.Required("first_name", "last_name")
	form.MinLength("first_name", 2, r)
	form.MinLength("last_name", 2, r)

	if !form.Valid() {
		data := make(map[string]interface{})
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		stringMap := make(map[string]string)
		stringMap["active_tab"] = ""

		render.Template(w, r, "profile.page.tmpl", &models.TemplateData{
			Form:      form,
			Data:      data,
			StringMap: stringMap,
		})
		return
	}

	user.Firstname = firstName
	user.LastName = lastName
	user.Phone = phone

	err = m.DB.UpdateUser(user)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Profil bilgileriniz güncellenirken bir hata oluştu"

		data := make(map[string]interface{})
		data["User"] = user

		render.Template(w, r, "profile.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      form,
		})
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Profil bilgileriniz başarıyla güncellendi")
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

func (m *Repository) PostPasswordUpdate(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		userID := m.App.Session.GetInt(r.Context(), "user_id")

		stringMap := make(map[string]string)
		stringMap["error"] = "Form işlenirken bir hata oluştu"
		stringMap["active_tab"] = "password"

		data := make(map[string]interface{})
		user, _ := m.DB.GetUserByID(userID)
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "password.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      forms.New(nil),
		})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	currentPassword := r.Form.Get("current_password")
	newPassword := r.Form.Get("new_password")
	confirmPassword := r.Form.Get("confirm_password")

	form := forms.New(r.PostForm)
	form.Required("current_password", "new_password", "confirm_password")
	form.MinLength("new_password", 8, r)

	if newPassword != confirmPassword {
		form.Errors.Add("confirm_password", "Şifreler eşleşmiyor")
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Kullanıcı bilgileri alınamadı"
		stringMap["active_tab"] = "password"

		data := make(map[string]interface{})
		render.Template(w, r, "password.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      form,
		})
		return
	}

	_, _, err = m.DB.Authenticate(user.Email, currentPassword)
	if err != nil {
		form.Errors.Add("current_password", "Mevcut şifreniz yanlış")
	}

	if !form.Valid() {
		data := make(map[string]interface{})
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		stringMap := make(map[string]string)
		stringMap["active_tab"] = "password"

		render.Template(w, r, "password.page.tmpl", &models.TemplateData{
			Form:      form,
			Data:      data,
			StringMap: stringMap,
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Şifreniz güncellenirken bir hata oluştu"
		stringMap["active_tab"] = "password"

		data := make(map[string]interface{})
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "password.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      form,
		})
		return
	}

	user.Password = string(hashedPassword)
	err = m.DB.UpdateUserPassword(user)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Şifreniz güncellenirken bir hata oluştu"

		data := make(map[string]interface{})
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "password.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      form,
		})
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Şifreniz başarıyla güncellendi")
	http.Redirect(w, r, "/user/password", http.StatusSeeOther)
}

func (m *Repository) PostPaymentAdd(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		userID := m.App.Session.GetInt(r.Context(), "user_id")

		stringMap := make(map[string]string)
		stringMap["error"] = "Form işlenirken bir hata oluştu"
		stringMap["active_tab"] = "payment"

		data := make(map[string]interface{})
		user, _ := m.DB.GetUserByID(userID)
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "payments.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      forms.New(nil),
		})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	cardName := r.Form.Get("card_name")
	cardNumber := r.Form.Get("card_number")
	expiryDate := r.Form.Get("expiry_date")
	cvv := r.Form.Get("cvv")

	form := forms.New(r.PostForm)
	form.Required("card_name", "card_number", "expiry_date", "cvv")

	cardNumber = strings.ReplaceAll(cardNumber, " ", "")
	if len(cardNumber) < 13 || len(cardNumber) > 19 {
		form.Errors.Add("card_number", "Geçerli bir kart numarası giriniz")
	}

	var expiryMonth, expiryYear int
	parts := strings.Split(expiryDate, "/")
	if len(parts) != 2 {
		form.Errors.Add("expiry_date", "Son kullanma tarihini MM/YY formatında giriniz")
	} else {
		var err error
		expiryMonth, err = strconv.Atoi(parts[0])
		if err != nil || expiryMonth < 1 || expiryMonth > 12 {
			form.Errors.Add("expiry_date", "Geçerli bir ay giriniz (01-12)")
		}

		expiryYear, err = strconv.Atoi(parts[1])
		if err != nil {
			form.Errors.Add("expiry_date", "Geçerli bir yıl giriniz")
		} else {
			if expiryYear < 100 {
				expiryYear += 2000
			}

			currentYear := time.Now().Year()
			currentMonth := int(time.Now().Month())

			if expiryYear < currentYear || (expiryYear == currentYear && expiryMonth < currentMonth) {
				form.Errors.Add("expiry_date", "Kartınızın süresi dolmuş")
			}
		}
	}

	if len(cvv) < 3 || len(cvv) > 4 {
		form.Errors.Add("cvv", "Geçerli bir CVV giriniz")
	}

	if !form.Valid() {
		user, _ := m.DB.GetUserByID(userID)
		data := make(map[string]interface{})
		data["User"] = user

		reservations, _ := m.DB.GetReservationsByUserID(userID)
		data["Reservations"] = reservations
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		stringMap := make(map[string]string)
		stringMap["active_tab"] = "payment"

		render.Template(w, r, "payments.page.tmpl", &models.TemplateData{
			Form:      form,
			Data:      data,
			StringMap: stringMap,
		})
		return
	}

	var cardType string
	if strings.HasPrefix(cardNumber, "4") {
		cardType = "Visa"
	} else if strings.HasPrefix(cardNumber, "5") {
		cardType = "MasterCard"
	} else if strings.HasPrefix(cardNumber, "3") {
		cardType = "American Express"
	} else if strings.HasPrefix(cardNumber, "9") {
		cardType = "Troy"
	} else {
		cardType = "Diğer"
	}

	lastFour := cardNumber[len(cardNumber)-4:]

	paymentMethod := models.PaymentMethod{
		UserID:      userID,
		CardName:    cardName,
		LastFour:    lastFour,
		ExpiryMonth: expiryMonth,
		ExpiryYear:  expiryYear,
		CardType:    cardType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err = m.DB.AddPaymentMethod(paymentMethod)
	if err != nil {
		stringMap := make(map[string]string)
		stringMap["error"] = "Ödeme yöntemi eklenirken bir hata oluştu"

		data := make(map[string]interface{})
		user, _ := m.DB.GetUserByID(userID)
		data["User"] = user
		paymentMethods, _ := m.DB.GetPaymentMethodsByUserID(userID)
		data["PaymentMethods"] = paymentMethods

		render.Template(w, r, "payments.page.tmpl", &models.TemplateData{
			StringMap: stringMap,
			Data:      data,
			Form:      form,
		})
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Ödeme yöntemi başarıyla eklendi")
	http.Redirect(w, r, "/user/payments", http.StatusSeeOther)
}

func (m *Repository) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Geçersiz ödeme yöntemi ID'si")
		http.Redirect(w, r, "/user/payments", http.StatusSeeOther)
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")

	isOwner, err := m.DB.IsPaymentMethodOwner(id, userID)
	if err != nil || !isOwner {
		m.App.Session.Put(r.Context(), "error", "Bu ödeme yöntemini silme yetkiniz yok")
		http.Redirect(w, r, "/user/payments", http.StatusSeeOther)
		return
	}

	err = m.DB.DeletePaymentMethod(id)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Ödeme yöntemi silinirken bir hata oluştu")
		http.Redirect(w, r, "/user/payments", http.StatusSeeOther)
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Ödeme yöntemi başarıyla silindi")
	http.Redirect(w, r, "/user/payments", http.StatusSeeOther)
}

func (m *Repository) UpdatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Form işlenirken bir hata oluştu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Oturum açmanız gerekiyor"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Geçersiz kart ID'si"})
		return
	}

	isOwner, err := m.DB.IsPaymentMethodOwner(id, userID)
	if err != nil || !isOwner {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Bu ödeme yöntemini güncelleme yetkiniz yok"})
		return
	}

	paymentMethod, err := m.DB.GetPaymentMethodByID(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Kart bilgileri alınamadı"})
		return
	}

	cardName := r.Form.Get("card_name")
	expiryDate := r.Form.Get("expiry_date")

	if cardName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Kart sahibi adı gereklidir"})
		return
	}

	var expiryMonth, expiryYear int
	parts := strings.Split(expiryDate, "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Son kullanma tarihini MM/YY formatında giriniz"})
		return
	}

	var err1, err2 error
	expiryMonth, err1 = strconv.Atoi(parts[0])
	expiryYear, err2 = strconv.Atoi(parts[1])

	if err1 != nil || err2 != nil || expiryMonth < 1 || expiryMonth > 12 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Geçersiz son kullanma tarihi"})
		return
	}

	if expiryYear < 100 {
		expiryYear += 2000
	}

	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	if expiryYear < currentYear || (expiryYear == currentYear && expiryMonth < currentMonth) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Kartınızın süresi dolmuş"})
		return
	}

	paymentMethod.CardName = cardName
	paymentMethod.ExpiryMonth = expiryMonth
	paymentMethod.ExpiryYear = expiryYear

	err = m.DB.UpdatePaymentMethod(paymentMethod)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Kart bilgileri güncellenirken bir hata oluştu"})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"success": "Kart bilgileri başarıyla güncellendi"})
}

func (m *Repository) CancelReservation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon ID'si")
		http.Redirect(w, r, "/user/reservations", http.StatusSeeOther)
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")

	isOwner, err := m.DB.IsReservationOwner(id, userID)
	if err != nil || !isOwner {
		m.App.Session.Put(r.Context(), "error", "Bu rezervasyonu iptal etme yetkiniz yok")
		http.Redirect(w, r, "/user/reservations", http.StatusSeeOther)
		return
	}

	err = m.DB.CancelReservation(id)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Rezervasyon iptal edilirken bir hata oluştu")
		http.Redirect(w, r, "/user/reservations", http.StatusSeeOther)
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Rezervasyon başarıyla iptal edildi")
	http.Redirect(w, r, "/user/reservations", http.StatusSeeOther)
}

func (m *Repository) PostContact(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "İletişim formu işlenemedi")
		http.Redirect(w, r, "/contact", http.StatusTemporaryRedirect)
		return
	}

	name := r.Form.Get("name")
	email := r.Form.Get("email")
	message := r.Form.Get("message")
	phone := r.Form.Get("phone")

	form := forms.New(r.PostForm)
	form.Required("name", "email", "message")
	form.MinLength("name", 3, r)
	form.IsEmail("email")

	if !form.Valid() {
		data := make(map[string]interface{})
		data["name"] = name
		data["email"] = email
		data["message"] = message
		data["phone"] = phone

		render.Template(w, r, "contact.page.tmpl", &models.TemplateData{
			Form: form,
			Data: data,
		})
		return
	}

	type ContactMailTemplateData struct {
		Name    string
		Email   string
		Phone   string
		Message string
		Year    int
	}

	currentYear := time.Now().Year()
	mailData := ContactMailTemplateData{
		Name:    html.EscapeString(name),
		Email:   html.EscapeString(email),
		Phone:   html.EscapeString(phone),
		Message: html.EscapeString(message),
		Year:    currentYear,
	}

	htmlMessage := ""
	tmpl, err := template.New("contact_mail").ParseFiles("./templates/contact-form.mail.tmpl")
	if err != nil {
		m.App.ErrorLog.Printf("Mail şablonu ayrıştırılamadı: %v\n", err)
		htmlMessage = fmt.Sprintf(`
	<h1>Yeni İletişim Mesajı</h1>
	<p>Gönderen: %s</p>
	<p>Email: %s</p>
	<p>Telefon: %s</p>
	<p>Mesaj:</p>
	<p>%s</p>
		`, mailData.Name, mailData.Email, mailData.Phone, mailData.Message)
	} else {
		buf := new(bytes.Buffer)
		err = tmpl.ExecuteTemplate(buf, "contact_form_mail", mailData)
		if err != nil {
			m.App.ErrorLog.Printf("Mail şablonu çalıştırılamadı: %v\n", err)
			htmlMessage = fmt.Sprintf(`
			<h1>Yeni İletişim Mesajı</h1>
			<p>Gönderen: %s</p>
			<p>Email: %s</p>
			<p>Telefon: %s</p>
			<p>Mesaj:</p>
			<p>%s</p>
			`, mailData.Name, mailData.Email, mailData.Phone, mailData.Message)
		} else {
			htmlMessage = buf.String()
		}
	}

	msg := models.MailData{
		To:      "admin@site.com",
		From:    email,
		Subject: "İletişim Formu Mesajı",
		Content: htmlMessage,
	}

	m.App.MailChan <- msg

	m.App.Session.Put(r.Context(), "flash", "Mesajınız başarıyla gönderildi. En kısa sürede size dönüş yapacağız.")
	http.Redirect(w, r, "/contact", http.StatusSeeOther)
}

func (m *Repository) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	reservations, err := m.DB.GetAllReservations()
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon bilgileri alınamadı: %v\n", err)
		reservations = []models.Reservations{}
	}

	totalReservations := len(reservations)

	rooms, err := m.DB.GetAllRooms()
	if err != nil {
		m.App.ErrorLog.Printf("Oda bilgileri alınamadı: %v\n", err)
		rooms = []models.Room{}
	}
	totalRooms := len(rooms)

	users, err := m.DB.GetAllUsers()
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		users = []models.User{}
	}
	totalUsers := len(users)

	var latestReservations []models.Reservations
	if len(reservations) > 5 {
		latestReservations = reservations[:5]
	} else {
		latestReservations = reservations
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Reservations"] = latestReservations
	data["TotalReservations"] = totalReservations
	data["TotalRooms"] = totalRooms
	data["TotalUsers"] = totalUsers

	stringMap := make(map[string]string)
	for _, res := range latestReservations {
		startDateStr := res.StartDate.Format("02.01.2006")
		endDateStr := res.EndDate.Format("02.01.2006")
		stringMap[fmt.Sprintf("start_date_%d", res.ID)] = startDateStr
		stringMap[fmt.Sprintf("end_date_%d", res.ID)] = endDateStr
	}

	render.Template(w, r, "admin-dashboard.page.tmpl", &models.TemplateData{
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) AdminAllReservations(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		pageNum, err := strconv.Atoi(pageStr)
		if err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	searchTerm := r.URL.Query().Get("search")

	itemsPerPage := 10

	allReservations, err := m.DB.GetAllReservations()
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon bilgileri alınamadı: %v\n", err)
		allReservations = []models.Reservations{}
	}

	var filteredReservations []models.Reservations
	if searchTerm != "" {
		searchTermLower := strings.ToLower(searchTerm)
		for _, res := range allReservations {
			if strings.Contains(strings.ToLower(res.FirstName), searchTermLower) ||
				strings.Contains(strings.ToLower(res.LastName), searchTermLower) ||
				strings.Contains(strings.ToLower(res.Email), searchTermLower) ||
				strings.Contains(strings.ToLower(res.Phone), searchTermLower) ||
				strings.Contains(strings.ToLower(res.Room.RoomName), searchTermLower) {
				filteredReservations = append(filteredReservations, res)
			}
		}
	} else {
		filteredReservations = allReservations
	}

	totalItems := len(filteredReservations)
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage

	start := (page - 1) * itemsPerPage
	end := start + itemsPerPage
	if end > totalItems {
		end = totalItems
	}

	var paginatedReservations []models.Reservations
	if start < totalItems {
		paginatedReservations = filteredReservations[start:end]
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Reservations"] = paginatedReservations
	data["CurrentPage"] = page
	data["TotalPages"] = totalPages
	data["TotalItems"] = totalItems
	data["SearchTerm"] = searchTerm

	stringMap := make(map[string]string)
	for _, res := range paginatedReservations {
		startDateStr := res.StartDate.Format("02.01.2006")
		endDateStr := res.EndDate.Format("02.01.2006")
		stringMap[fmt.Sprintf("start_date_%d", res.ID)] = startDateStr
		stringMap[fmt.Sprintf("end_date_%d", res.ID)] = endDateStr
	}

	render.Template(w, r, "admin-reservations.page.tmpl", &models.TemplateData{
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) AdminReservationDetail(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon ID'si")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	allReservations, err := m.DB.GetAllReservations()
	if err != nil {
		m.App.ErrorLog.Printf("Tüm rezervasyon bilgileri alınamadı: %v\n", err)
		allReservations = []models.Reservations{}
	}

	var reservation models.Reservations
	found := false
	for _, res := range allReservations {
		if res.ID == id {
			reservation = res
			found = true
			break
		}
	}

	if !found {
		m.App.ErrorLog.Printf("Rezervasyon bulunamadı: ID=%d\n", id)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bulunamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	paymentInfo, err := m.DB.GetReservationPayStatusByReservationID(id)
	if err != nil {
		m.App.ErrorLog.Printf("Ödeme bilgileri alınamadı: %v\n", err)

		roomInfo, roomErr := m.DB.GetRoomInfoByRoomID(reservation.RoomID)
		var totalAmount int = 0
		if roomErr == nil {
			days := int(reservation.EndDate.Sub(reservation.StartDate).Hours() / 24)
			if days <= 0 {
				days = 1
			}
			totalAmount = days * roomInfo.RoomDailyPrice
		}

		paymentInfo = models.ReservationPayStatus{
			ReservationID: id,
			TotalAmount:   totalAmount,
			PaymentStatus: "pending",
			PaymentMethod: "later",
		}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Reservation"] = reservation
	data["PaymentInfo"] = paymentInfo

	stringMap := make(map[string]string)

	startDateStr := reservation.StartDate.Format("2006-01-02")
	endDateStr := reservation.EndDate.Format("2006-01-02")
	displayStartDate := reservation.StartDate.Format("02.01.2006")
	displayEndDate := reservation.EndDate.Format("02.01.2006")

	stringMap["start_date"] = startDateStr
	stringMap["end_date"] = endDateStr
	stringMap[fmt.Sprintf("start_date_%d", reservation.ID)] = displayStartDate
	stringMap[fmt.Sprintf("end_date_%d", reservation.ID)] = displayEndDate

	var roomReservations []map[string]interface{}
	for _, res := range allReservations {
		// Çakışmaları önlemek için tüm rezervasyonları göster (sadece reddedilenler hariç)
		if res.RoomID == reservation.RoomID && res.ID != reservation.ID && res.ReservationStatus != "rejected" {
			resData := map[string]interface{}{
				"id":         res.ID,
				"room_id":    res.RoomID,
				"start_date": res.StartDate.Format("2006-01-02"),
				"end_date":   res.EndDate.Format("2006-01-02"),
				"first_name": res.FirstName,
				"last_name":  res.LastName,
				"email":      res.Email,
				"status":     res.ReservationStatus,
			}
			roomReservations = append(roomReservations, resData)
		}
	}

	roomReservationsJSON, err := json.Marshal(roomReservations)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon verileri JSON formatına dönüştürülemedi: %v\n", err)
	} else {
		stringMap["reservations_json"] = string(roomReservationsJSON)
		m.App.InfoLog.Printf("Aynı odaya ait %d rezervasyon bulundu\n", len(roomReservations))
	}

	render.Template(w, r, "admin-reservation-detail.page.tmpl", &models.TemplateData{
		StringMap: stringMap,
		Data:      data,
	})
}

func (m *Repository) AdminUpdateReservationStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon ID'si")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	status := chi.URLParam(r, "status")
	if status != "accepted" && status != "rejected" {
		m.App.ErrorLog.Printf("Geçersiz rezervasyon durumu: %s\n", status)
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon durumu")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", id), http.StatusSeeOther)
		return
	}

	allReservations, err := m.DB.GetAllReservations()
	if err != nil {
		m.App.ErrorLog.Printf("Tüm rezervasyon bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgisi alınamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	var reservation models.Reservations
	found := false
	for _, res := range allReservations {
		if res.ID == id {
			reservation = res
			found = true
			break
		}
	}

	if !found {
		m.App.ErrorLog.Printf("Rezervasyon bulunamadı: ID=%d\n", id)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bulunamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	err = m.DB.UpdateReservationStatus(id, status)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon durumu güncellenirken hata oluştu: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon durumu güncellenirken hata oluştu")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", id), http.StatusSeeOther)
		return
	}

	if status == "rejected" {
		err = m.DB.DeleteRoomRestrictionByReservationID(id)
		if err != nil {
			m.App.ErrorLog.Printf("Reddedilen rezervasyon için oda kısıtlaması kaldırılamadı: %v\n", err)
		} else {
			m.App.InfoLog.Printf("Reddedilen rezervasyon için oda kısıtlaması başarıyla kaldırıldı: ID=%d\n", id)
		}
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(reservation.RoomID)
	if err != nil {
		roomInfo = models.RoomInfo{
			RoomMaxCap:     2,
			RoomDailyPrice: 500,
		}
	}

	totalDays := int(reservation.EndDate.Sub(reservation.StartDate).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}
	totalAmount := totalDays * roomInfo.RoomDailyPrice

	var paymentStatus, paymentMethod string
	var paymentDate *time.Time

	paymentInfo, err := m.DB.GetReservationPayStatusByReservationID(reservation.ID)
	if err != nil {
		paymentStatus = reservation.PaymentStatus
		paymentMethod = reservation.PaymentMethod
		paymentDate = nil
	} else {
		paymentStatus = paymentInfo.PaymentStatus
		paymentMethod = paymentInfo.PaymentMethod
		paymentDate = paymentInfo.PaymentDate
	}

	type MailTemplateData struct {
		FirstName string
		LastName  string
		RoomName  string
		StartDate string

		EndDate       string
		Phone         string
		Email         string
		Year          int
		Capacity      int
		Nights        int
		DailyPrice    int
		TotalPrice    int
		PaymentStatus string
		PaymentMethod string
		PaymentDate   string
	}

	mailData := MailTemplateData{
		FirstName:     reservation.FirstName,
		LastName:      reservation.LastName,
		RoomName:      reservation.Room.RoomName,
		StartDate:     reservation.StartDate.Format("2006-01-02"),
		EndDate:       reservation.EndDate.Format("2006-01-02"),
		Phone:         reservation.Phone,
		Email:         reservation.Email,
		Year:          time.Now().Year(),
		Capacity:      roomInfo.RoomMaxCap,
		Nights:        totalDays,
		DailyPrice:    roomInfo.RoomDailyPrice,
		TotalPrice:    totalAmount,
		PaymentStatus: paymentStatus,
		PaymentMethod: paymentMethod,
		PaymentDate: func() string {
			if paymentDate != nil {
				return paymentDate.Format("2006-01-02")
			}
			return "Henüz ödenmedi"
		}(),
	}

	var tpl bytes.Buffer
	var templateFile string

	if status == "accepted" {
		templateFile = "./templates/reservation-confirmation.mail.tmpl"
	} else {
		templateFile = "./templates/reservation-rejected.mail.tmpl"
	}

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		m.App.ErrorLog.Printf("Mail şablonu yüklenemedi: %v", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon durum maili gönderilemedi")
	} else {
		err = tmpl.Execute(&tpl, mailData)
		if err != nil {
			m.App.ErrorLog.Printf("Mail şablonu işlenemedi: %v", err)
			m.App.Session.Put(r.Context(), "error", "Rezervasyon durum maili gönderilemedi")
		} else {
			var subject string
			if status == "accepted" {
				subject = "Rezervasyon Onaylandı"
			} else {
				subject = "Rezervasyon Reddedildi"
			}

			msg := models.MailData{
				To:      reservation.Email,
				From:    "muhammed@here.com",
				Subject: subject,
				Content: tpl.String(),
			}
			m.App.MailChan <- msg
		}
	}

	var flashMsg string
	if status == "accepted" {
		flashMsg = "Rezervasyon başarıyla onaylandı."
	} else {
		flashMsg = "Rezervasyon başarıyla reddedildi."
	}

	m.App.Session.Put(r.Context(), "flash", flashMsg)
	http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", id), http.StatusSeeOther)
}

func (m *Repository) AdminDeleteReservation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon ID'si")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	err = m.DB.CancelReservation(id)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon iptal edilemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon iptal edilemedi")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Rezervasyon başarıyla silindi")
	}

	http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
}

func (m *Repository) AdminRooms(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rooms, err := m.DB.GetAllRooms()
	if err != nil {
		m.App.ErrorLog.Printf("Oda bilgileri alınamadı: %v\n", err)
		rooms = []models.Room{}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Rooms"] = rooms

	stringMap := make(map[string]string)
	for _, room := range rooms {
		createdAt := room.CreatedAt.Format("02.01.2006")
		stringMap[fmt.Sprintf("created_at_%d", room.ID)] = createdAt
		updatedAt := room.UpdatedAt.Format("02.01.2006")
		stringMap[fmt.Sprintf("updated_at_%d", room.ID)] = updatedAt
	}

	render.Template(w, r, "admin-rooms.page.tmpl", &models.TemplateData{
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) AdminAddRoom(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form ayrıştırılamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	roomName := r.Form.Get("room_name")
	if roomName == "" {
		m.App.Session.Put(r.Context(), "error", "Oda adı boş olamaz")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	room := models.Room{
		RoomName: roomName,
	}

	_, err = m.DB.InsertRoom(room)
	if err != nil {
		m.App.ErrorLog.Printf("Oda eklenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Oda eklenirken bir hata oluştu")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Oda başarıyla eklendi")
	}

	http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
}

func (m *Repository) AdminUpdateRoom(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form ayrıştırılamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	roomID, err := strconv.Atoi(r.Form.Get("room_id"))
	if err != nil {
		m.App.ErrorLog.Printf("Oda ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz oda ID'si")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	roomName := r.Form.Get("room_name")
	if roomName == "" {
		m.App.Session.Put(r.Context(), "error", "Oda adı boş olamaz")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	room := models.Room{
		ID:       roomID,
		RoomName: roomName,
	}

	err = m.DB.UpdateRoom(room)
	if err != nil {
		m.App.ErrorLog.Printf("Oda güncellenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Oda güncellenirken bir hata oluştu")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Oda başarıyla güncellendi")
	}

	http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
}

func (m *Repository) AdminDeleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Oda ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz oda ID'si")
		http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
		return
	}

	err = m.DB.DeleteRoom(id)
	if err != nil {
		m.App.ErrorLog.Printf("Oda silinemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Oda silinirken bir hata oluştu")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Oda başarıyla silindi")
	}

	http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
}

func (m *Repository) AdminUsers(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		pageNum, err := strconv.Atoi(pageStr)
		if err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	searchTerm := r.URL.Query().Get("search")

	itemsPerPage := 10

	allUsers, err := m.DB.GetAllUsers()
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		allUsers = []models.User{}
	}

	var filteredUsers []models.User
	if searchTerm != "" {
		searchTermLower := strings.ToLower(searchTerm)
		for _, u := range allUsers {
			if strings.Contains(strings.ToLower(u.Firstname), searchTermLower) ||
				strings.Contains(strings.ToLower(u.LastName), searchTermLower) ||
				strings.Contains(strings.ToLower(u.Email), searchTermLower) {
				filteredUsers = append(filteredUsers, u)
			}
		}
	} else {
		filteredUsers = allUsers
	}

	totalItems := len(filteredUsers)
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage

	start := (page - 1) * itemsPerPage
	end := start + itemsPerPage
	if end > totalItems {
		end = totalItems
	}

	var paginatedUsers []models.User
	if start < totalItems {
		paginatedUsers = filteredUsers[start:end]
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Users"] = paginatedUsers
	data["CurrentPage"] = page
	data["TotalPages"] = totalPages
	data["TotalItems"] = totalItems
	data["SearchTerm"] = searchTerm

	stringMap := make(map[string]string)
	for _, u := range paginatedUsers {
		createdAt := u.CreatedAt.Format("02.01.2006")
		stringMap[fmt.Sprintf("created_at_%d", u.ID)] = createdAt
	}

	render.Template(w, r, "admin-users.page.tmpl", &models.TemplateData{
		Data:      data,
		StringMap: stringMap,
	})
}

func (m *Repository) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form ayrıştırılamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	userID, err := strconv.Atoi(r.Form.Get("user_id"))
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz kullanıcı ID'si")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	firstName := r.Form.Get("first_name")
	lastName := r.Form.Get("last_name")
	email := r.Form.Get("email")
	phone := r.Form.Get("phone")
	accessLevel, err := strconv.Atoi(r.Form.Get("access_level"))
	if err != nil {
		accessLevel = 1
	}

	if firstName == "" || lastName == "" || email == "" {
		m.App.Session.Put(r.Context(), "error", "Ad, soyad ve email alanları zorunludur")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	user := models.User{
		ID:           userID,
		Firstname:    firstName,
		LastName:     lastName,
		Email:        email,
		Phone:        phone,
		Accsesslevel: accessLevel,
	}

	err = m.DB.UpdateUser(user)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı güncellenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı güncellenirken bir hata oluştu")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Kullanıcı başarıyla güncellendi")
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (m *Repository) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz kullanıcı ID'si")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	currentUserID := m.App.Session.GetInt(r.Context(), "user_id")
	if id == currentUserID {
		m.App.Session.Put(r.Context(), "error", "Kendi hesabınızı silemezsiniz")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	err = m.DB.DeleteUser(id)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı silinemedi: %v\n", err)
		if err.Error() == "bu kullanıcıya ait rezervasyonlar bulunduğu için kullanıcı silinemez" {
			m.App.Session.Put(r.Context(), "error", "Bu kullanıcının rezervasyonu olduğundan silinemedi.")
		} else {
			m.App.Session.Put(r.Context(), "error", "Kullanıcı silinirken bir hata oluştu")
		}
	} else {
		m.App.Session.Put(r.Context(), "flash", "Kullanıcı başarıyla silindi")
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (m *Repository) TestPage(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "activation-result.page.tmpl", &models.TemplateData{
		Data: map[string]interface{}{"Success": true},
	})
}

func (m *Repository) AdminUpdateReservation(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form verileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form verileri alınamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	reservationID, err := strconv.Atoi(r.Form.Get("reservation_id"))
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz rezervasyon ID'si")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	reservations, err := m.DB.GetAllReservations()
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bilgileri alınamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	var reservation models.Reservations
	found := false
	for _, res := range reservations {
		if res.ID == reservationID {
			reservation = res
			found = true
			break
		}
	}

	if !found {
		m.App.ErrorLog.Printf("Rezervasyon bulunamadı: ID=%d\n", reservationID)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon bulunamadı")
		http.Redirect(w, r, "/admin/reservations", http.StatusSeeOther)
		return
	}

	startDate := r.Form.Get("start_date")
	endDate := r.Form.Get("end_date")
	layout := "2006-01-02"

	newStartDate, err := time.Parse(layout, startDate)
	if err != nil {
		m.App.ErrorLog.Printf("Başlangıç tarihi dönüştürülemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz başlangıç tarihi")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
		return
	}

	newEndDate, err := time.Parse(layout, endDate)
	if err != nil {
		m.App.ErrorLog.Printf("Bitiş tarihi dönüştürülemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz bitiş tarihi")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
		return
	}

	for _, res := range reservations {
		if res.ID == reservationID {
			continue
		}

		if res.RoomID != reservation.RoomID {
			continue
		}

		if res.ReservationStatus == "rejected" {
			continue
		}

		if (newStartDate.Before(res.EndDate) || newStartDate.Equal(res.EndDate)) &&
			(newEndDate.After(res.StartDate) || newEndDate.Equal(res.StartDate)) {
			m.App.ErrorLog.Printf("Tarih çakışması: %v-%v ve %v-%v\n",
				newStartDate, newEndDate, res.StartDate, res.EndDate)
			m.App.Session.Put(r.Context(), "error", "Seçilen tarihler başka bir rezervasyonla çakışıyor (onay bekleyen veya onaylanmış)")
			http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
			return
		}
	}

	// Eski tarih bilgilerini kaydet (UpdateReservation çağrılmadan önce)
	oldStartDate := reservation.StartDate
	oldEndDate := reservation.EndDate

	err = m.DB.DeleteRoomRestrictionByReservationID(reservationID)
	if err != nil {
		m.App.ErrorLog.Printf("Eski rezervasyon kısıtlaması silinemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon güncellenirken bir hata oluştu")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
		return
	}

	reservation.StartDate = newStartDate
	reservation.EndDate = newEndDate

	err = m.DB.UpdateReservation(reservation)
	if err != nil {
		m.App.ErrorLog.Printf("Rezervasyon güncellenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon güncellenirken bir hata oluştu")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
		return
	}

	restriction := models.RoomRestrictions{
		StartDate:     newStartDate,
		EndDate:       newEndDate,
		RoomID:        reservation.RoomID,
		ReservationID: reservationID,
		RestrictionID: 1,
	}

	err = m.DB.InsertRoomRestrictions(restriction)
	if err != nil {
		m.App.ErrorLog.Printf("Yeni rezervasyon kısıtlaması eklenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Rezervasyon güncellenirken bir hata oluştu")
		http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(reservation.RoomID)
	if err != nil {
		m.App.ErrorLog.Printf("Oda bilgileri alınamadı: %v\n", err)
		roomInfo = models.RoomInfo{
			RoomDailyPrice: 500,
		}
	}

	originalDays := int(oldEndDate.Sub(oldStartDate).Hours() / 24)
	if originalDays <= 0 {
		originalDays = 1
	}

	newDays := int(newEndDate.Sub(newStartDate).Hours() / 24)
	if newDays <= 0 {
		newDays = 1
	}

	oldTotalAmount := originalDays * roomInfo.RoomDailyPrice
	newTotalAmount := newDays * roomInfo.RoomDailyPrice

	priceDifference := newTotalAmount - oldTotalAmount

	paymentInfo, err := m.DB.GetReservationPayStatusByReservationID(reservationID)
	if err != nil {
		m.App.ErrorLog.Printf("Ödeme bilgileri alınamadı: %v\n", err)
	} else {
		wasAlreadyPaid := paymentInfo.PaymentStatus == "paid"

		paymentInfo.TotalAmount = newTotalAmount

		if wasAlreadyPaid && priceDifference != 0 {
			allUsers, err := m.DB.GetAllUsers()
			if err != nil {
				m.App.ErrorLog.Printf("Kullanıcılar alınamadı: %v\n", err)
			} else {
				for _, user := range allUsers {
					if user.Email == reservation.Email {
						currentUser, err := m.DB.GetUserByID(user.ID)
						if err != nil {
							m.App.ErrorLog.Printf("Güncel kullanıcı bilgileri alınamadı: %v\n", err)
							break
						}

						var newBalance int
						var balanceMessage string
						var shouldSetPending bool = false

						if priceDifference < 0 {
							refundAmount := -priceDifference
							newBalance = currentUser.Balance + refundAmount
							balanceMessage = fmt.Sprintf(" İade edilen tutar (%d ₺) bakiyenize eklenmiştir.", refundAmount)

						} else {

							newBalance = currentUser.Balance
							balanceMessage = fmt.Sprintf(" Ek tutar (%d ₺) için ödeme bekleniyor.", priceDifference)
							shouldSetPending = true
							paymentInfo.PreviouslyPaid = oldTotalAmount
						}

						m.App.InfoLog.Printf("Kullanıcı %s için bakiye güncelleniyor: %d -> %d (fark: %d)", currentUser.Email, currentUser.Balance, newBalance, priceDifference)

						err = m.DB.UpdateUserBalance(currentUser.ID, newBalance)
						if err != nil {
							m.App.ErrorLog.Printf("Kullanıcı bakiyesi güncellenemedi: %v\n", err)
						} else {
							if shouldSetPending {
								paymentInfo.PaymentStatus = "pending"
								paymentInfo.PaymentMethod = "later"
								paymentInfo.PaymentDate = nil

								m.App.Session.Put(r.Context(), "flash", fmt.Sprintf("Rezervasyon tarihleri güncellendi.%s Rezervasyon yeniden onay bekliyor.", balanceMessage))

								err = m.DB.UpdateReservationStatus(reservationID, "pending")
								if err != nil {
									m.App.ErrorLog.Printf("Rezervasyon durumu güncellenemedi: %v\n", err)
								}
							} else {
								m.App.Session.Put(r.Context(), "flash", fmt.Sprintf("Rezervasyon tarihleri güncellendi.%s", balanceMessage))
							}

							err = m.DB.UpdateReservationPayStatus(paymentInfo)
							if err != nil {
								m.App.ErrorLog.Printf("Ödeme bilgileri güncellenemedi: %v\n", err)
							}

							http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
							return
						}
						break
					}
				}
			}
		} else {
			paymentInfo.PaymentStatus = "pending"
			paymentInfo.PaymentMethod = "later"
			paymentInfo.PaymentDate = nil

			err = m.DB.UpdateReservationPayStatus(paymentInfo)
			if err != nil {
				m.App.ErrorLog.Printf("Ödeme bilgileri güncellenemedi: %v\n", err)
			}

			err = m.DB.UpdateReservationStatus(reservationID, "pending")
			if err != nil {
				m.App.ErrorLog.Printf("Rezervasyon durumu güncellenemedi: %v\n", err)
			}
		}
	}

	m.App.Session.Put(r.Context(), "flash", "Rezervasyon tarihleri güncellendi.")
	http.Redirect(w, r, fmt.Sprintf("/admin/reservations/%d", reservationID), http.StatusSeeOther)
}

func (m *Repository) Staff(w http.ResponseWriter, r *http.Request) {
	staff, err := m.DB.GetAllStaff()
	if err != nil {
		m.App.ErrorLog.Printf("Personel bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := make(map[string]interface{})
	data["Staff"] = staff

	render.Template(w, r, "staff.page.tmpl", &models.TemplateData{
		Data: data,
	})
}

func (m *Repository) StaffDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Personel ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz personel ID'si")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}

	staff, err := m.DB.GetStaffByID(id)
	if err != nil {
		m.App.ErrorLog.Printf("Personel bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel bilgileri alınamadı")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}

	data := make(map[string]interface{})
	data["Staff"] = staff

	render.Template(w, r, "staff-detail.page.tmpl", &models.TemplateData{
		Data: data,
	})
}
func (m *Repository) AdminStaff(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	staff, err := m.DB.GetAllStaff()
	if err != nil {
		m.App.ErrorLog.Printf("Personel bilgileri alınamadı: %v\n", err)
		staff = []models.StaffInfo{}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Staff"] = staff

	render.Template(w, r, "admin-staff.page.tmpl", &models.TemplateData{
		Data: data,
		Form: forms.New(nil),
	})
}

func (m *Repository) AdminAddStaff(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form ayrıştırılamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("first_name", "last_name", "email", "staff_rank")
	form.MinLength("first_name", 2, r)
	form.MinLength("last_name", 2, r)
	form.IsEmail("email")

	if !form.Valid() {
		userID := m.App.Session.GetInt(r.Context(), "user_id")
		user, _ := m.DB.GetUserByID(userID)
		staff, _ := m.DB.GetAllStaff()

		data := make(map[string]interface{})
		data["User"] = user
		data["Staff"] = staff

		render.Template(w, r, "admin-staff.page.tmpl", &models.TemplateData{
			Form: form,
			Data: data,
		})
		return
	}

	staffInfo := models.StaffInfo{
		FirstName: r.Form.Get("first_name"),
		LastName:  r.Form.Get("last_name"),
		Email:     r.Form.Get("email"),
		Phone:     r.Form.Get("phone"),
		StaffRank: r.Form.Get("staff_rank"),
		Floor:     r.Form.Get("floor"),
		Bio:       r.Form.Get("bio"),
		PhotoURL:  r.Form.Get("photo_url"),
	}

	_, err = m.DB.InsertStaff(staffInfo)
	if err != nil {
		m.App.ErrorLog.Printf("Personel eklenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel eklenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Personel başarıyla eklendi")
	http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
}

func (m *Repository) AdminUpdateStaff(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.ErrorLog.Printf("Form ayrıştırılamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Form işlenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	staffID, err := strconv.Atoi(r.Form.Get("staff_id"))
	if err != nil {
		m.App.ErrorLog.Printf("Personel ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz personel ID'si")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	staffInfo, err := m.DB.GetStaffByID(staffID)
	if err != nil {
		m.App.ErrorLog.Printf("Personel bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel bilgileri alınamadı")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("first_name", "last_name", "email", "staff_rank")
	form.MinLength("first_name", 2, r)
	form.MinLength("last_name", 2, r)
	form.IsEmail("email")

	if !form.Valid() {
		userID := m.App.Session.GetInt(r.Context(), "user_id")
		user, _ := m.DB.GetUserByID(userID)
		staff, _ := m.DB.GetAllStaff()

		data := make(map[string]interface{})
		data["User"] = user
		data["Staff"] = staff
		data["StaffEdit"] = staffInfo

		render.Template(w, r, "admin-staff.page.tmpl", &models.TemplateData{
			Form: form,
			Data: data,
		})
		return
	}

	staffInfo.FirstName = r.Form.Get("first_name")
	staffInfo.LastName = r.Form.Get("last_name")
	staffInfo.Email = r.Form.Get("email")
	staffInfo.Phone = r.Form.Get("phone")
	staffInfo.StaffRank = r.Form.Get("staff_rank")
	staffInfo.Floor = r.Form.Get("floor")
	staffInfo.Bio = r.Form.Get("bio")

	if r.Form.Get("photo_url") != "" {
		staffInfo.PhotoURL = r.Form.Get("photo_url")
	}

	err = m.DB.UpdateStaff(staffInfo)
	if err != nil {
		m.App.ErrorLog.Printf("Personel güncellenemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel güncellenirken bir hata oluştu")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	m.App.Session.Put(r.Context(), "flash", "Personel başarıyla güncellendi")
	http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
}

func (m *Repository) AdminDeleteStaff(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Personel ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz personel ID'si")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	err = m.DB.DeleteStaff(id)
	if err != nil {
		m.App.ErrorLog.Printf("Personel silinemedi: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel silinirken bir hata oluştu")
	} else {
		m.App.Session.Put(r.Context(), "flash", "Personel başarıyla silindi")
	}

	http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
}

func (m *Repository) AdminEditStaff(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Kullanıcı bilgileri alınamadı")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.ErrorLog.Printf("Personel ID'si alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Geçersiz personel ID'si")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	staffInfo, err := m.DB.GetStaffByID(id)
	if err != nil {
		m.App.ErrorLog.Printf("Personel bilgileri alınamadı: %v\n", err)
		m.App.Session.Put(r.Context(), "error", "Personel bilgileri alınamadı")
		http.Redirect(w, r, "/admin/staff", http.StatusSeeOther)
		return
	}

	staff, err := m.DB.GetAllStaff()
	if err != nil {
		m.App.ErrorLog.Printf("Tüm personel bilgileri alınamadı: %v\n", err)
		staff = []models.StaffInfo{}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Staff"] = staff
	data["StaffEdit"] = staffInfo

	render.Template(w, r, "admin-staff.page.tmpl", &models.TemplateData{
		Form: forms.New(nil),
		Data: data,
	})
}

func (m *Repository) Room(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Geçersiz oda numarası")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	room, err := m.DB.GetRoomByID(roomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Oda bulunamadı")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(roomID)
	if err != nil {
		m.App.ErrorLog.Printf("Oda detayları bulunamadı, varsayılan bilgiler kullanılıyor: %v", err)
		roomInfo = models.RoomInfo{
			RoomsID:                 roomID,
			FirstPicURL:             "/static/images/default-room.jpg",
			FirstText:               "Bu oda için henüz detaylı bilgi girilmemiştir.",
			FirstTittleFontawesome:  "fas fa-home",
			FirstTittle:             "Konfor",
			FirstTittleText:         "Konforlu bir oda",
			SecondTittleFontawesome: "fas fa-wifi",
			SecondTittle:            "Ücretsiz WiFi",
			SecondTittleText:        "Yüksek hızlı internet",
			ThirdTittleFontawesome:  "fas fa-utensils",
			ThirdTittle:             "Kahvaltı",
			ThirdTittleText:         "Kahvaltı dahil",
			RoomMaxCap:              2,
			RoomDailyPrice:          500,
			Room:                    room,
		}
	}

	data := make(map[string]interface{})
	data["Room"] = room
	data["RoomInfo"] = roomInfo

	render.Template(w, r, "room.page.tmpl", &models.TemplateData{
		Data: data,
	})
}

func (m *Repository) AdminEditRoomInfo(w http.ResponseWriter, r *http.Request) {
	userID := m.App.Session.GetInt(r.Context(), "user_id")
	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		m.App.ErrorLog.Printf("Kullanıcı bilgileri alınamadı: %v", err)
	}

	roomID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Geçersiz oda numarası")
		http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
		return
	}

	room, err := m.DB.GetRoomByID(roomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Oda bulunamadı")
		http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(roomID)
	if err != nil {
		roomInfo = models.RoomInfo{
			RoomsID: roomID,
			Room:    room,
		}
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["Room"] = room
	data["RoomInfo"] = roomInfo
	render.Template(w, r, "admin-edit-roominfo.page.tmpl", &models.TemplateData{
		Form: forms.New(nil),
		Data: data,
	})
}

func (m *Repository) AdminUpdateRoomInfo(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Form verileri alınamadı")
		http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
		return
	}

	roomID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Geçersiz oda numarası")
		http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
		return
	}

	room, err := m.DB.GetRoomByID(roomID)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "Oda bulunamadı")
		http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
		return
	}

	roomInfo, err := m.DB.GetRoomInfoByRoomID(roomID)
	isNew := false
	if err != nil {
		isNew = true
		roomInfo.RoomsID = roomID
	}

	roomInfo.FirstPicURL = r.Form.Get("first_pic_url")
	roomInfo.FirstText = r.Form.Get("first_text")
	roomInfo.FirstTittleFontawesome = r.Form.Get("first_tittle_fontawesome")
	roomInfo.FirstTittle = r.Form.Get("first_tittle")
	roomInfo.FirstTittleText = r.Form.Get("first_tittle_text")
	roomInfo.SecondTittleFontawesome = r.Form.Get("second_tittle_fontawesome")
	roomInfo.SecondTittle = r.Form.Get("second_tittle")
	roomInfo.SecondTittleText = r.Form.Get("second_tittle_text")
	roomInfo.ThirdTittleFontawesome = r.Form.Get("third_tittle_fontawesome")
	roomInfo.ThirdTittle = r.Form.Get("third_tittle")
	roomInfo.ThirdTittleText = r.Form.Get("third_tittle_text")

	roomMaxCap, err := strconv.Atoi(r.Form.Get("room_max_cap"))
	if err != nil {
		roomMaxCap = 2
	}
	roomInfo.RoomMaxCap = roomMaxCap

	roomDailyPrice, err := strconv.Atoi(r.Form.Get("room_daily_price"))
	if err != nil {
		roomDailyPrice = 0
	}
	roomInfo.RoomDailyPrice = roomDailyPrice

	form := forms.New(r.PostForm)
	form.Required("first_pic_url", "first_text")

	if !form.Valid() {
		data := make(map[string]interface{})
		data["Room"] = room
		data["RoomInfo"] = roomInfo

		render.Template(w, r, "admin-edit-roominfo.page.tmpl", &models.TemplateData{
			Form: form,
			Data: data,
		})
		return
	}

	if isNew {
		_, err = m.DB.InsertRoomInfo(roomInfo)
		if err != nil {
			m.App.Session.Put(r.Context(), "error", "Oda detayları kaydedilemedi: "+err.Error())
			http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
			return
		}
		m.App.Session.Put(r.Context(), "flash", "Oda detayları başarıyla eklendi")
	} else {
		err = m.DB.UpdateRoomInfo(roomInfo)
		if err != nil {
			m.App.Session.Put(r.Context(), "error", "Oda detayları güncellenemedi: "+err.Error())
			http.Redirect(w, r, "/admin/rooms", http.StatusTemporaryRedirect)
			return
		}
		m.App.Session.Put(r.Context(), "flash", "Oda detayları başarıyla güncellendi")
	}

	http.Redirect(w, r, "/admin/rooms", http.StatusSeeOther)
}

func (m *Repository) GetReservationAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Geçersiz rezervasyon ID'si"})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Oturum açmanız gerekiyor"})
		return
	}

	isOwner, err := m.DB.IsReservationOwner(id, userID)
	if err != nil || !isOwner {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Bu rezervasyona erişim yetkiniz yok"})
		return
	}

	paymentInfo, err := m.DB.GetReservationPayStatusByReservationID(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Rezervasyon bilgileri alınamadı"})
		return
	}

	response := map[string]interface{}{
		"id":           id,
		"total_amount": paymentInfo.TotalAmount,
		"status":       paymentInfo.PaymentStatus,
		"method":       paymentInfo.PaymentMethod,
	}

	json.NewEncoder(w).Encode(response)
}

func (m *Repository) GetUserPaymentMethodsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Oturum açmanız gerekiyor"})
		return
	}

	paymentMethods, err := m.DB.GetPaymentMethodsByUserID(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ödeme yöntemleri alınamadı"})
		return
	}

	var response []map[string]interface{}
	for _, pm := range paymentMethods {
		response = append(response, map[string]interface{}{
			"id":        pm.ID,
			"card_type": pm.CardType,
			"last_four": pm.LastFour,
		})
	}

	json.NewEncoder(w).Encode(response)
}

func (m *Repository) PayReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Form verileri alınamadı",
		})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Oturum açmanız gerekiyor",
		})
		return
	}

	reservationID, err := strconv.Atoi(r.Form.Get("reservation_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Geçersiz rezervasyon ID'si",
		})
		return
	}

	totalAmount, err := strconv.Atoi(r.Form.Get("total_amount"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Geçersiz tutar",
		})
		return
	}

	paymentMethod := r.Form.Get("payment_method")

	isOwner, err := m.DB.IsReservationOwner(reservationID, userID)
	if err != nil || !isOwner {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Bu rezervasyona erişim yetkiniz yok",
		})
		return
	}

	payStatus, err := m.DB.GetReservationPayStatusByReservationID(reservationID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Ödeme bilgileri alınamadı",
		})
		return
	}

	actualAmountToPay := totalAmount
	if payStatus.PreviouslyPaid > 0 {
		actualAmountToPay = totalAmount - payStatus.PreviouslyPaid
		if actualAmountToPay < 0 {
			actualAmountToPay = 0
		}
		m.App.InfoLog.Printf("Rezervasyon %d için ek ödeme: %d ₺ (Toplam: %d ₺, Önceki: %d ₺)",
			reservationID, actualAmountToPay, totalAmount, payStatus.PreviouslyPaid)
	}

	if paymentMethod == "balance" {
		user, err := m.DB.GetUserByID(userID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Kullanıcı bilgileri alınamadı",
			})
			return
		}

		if user.Balance < actualAmountToPay {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Yetersiz bakiye. Gerekli tutar: %d ₺", actualAmountToPay),
			})
			return
		}

		newBalance := user.Balance - actualAmountToPay
		err = m.DB.UpdateUserBalance(userID, newBalance)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Bakiye güncellenirken hata oluştu",
			})
			return
		}
	}

	now := time.Now()
	payStatus.PaymentStatus = "paid"
	payStatus.PaymentMethod = paymentMethod
	payStatus.PaymentDate = &now
	payStatus.PreviouslyPaid = 0

	err = m.DB.UpdateReservationPayStatus(payStatus)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Ödeme durumu güncellenirken hata oluştu",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Ödeme başarıyla tamamlandı",
	})
}

func (m *Repository) AddBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Form verileri alınamadı",
		})
		return
	}

	userID := m.App.Session.GetInt(r.Context(), "user_id")
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Oturum açmanız gerekiyor",
		})
		return
	}

	amount, err := strconv.Atoi(r.Form.Get("amount"))
	if err != nil || amount < 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Geçersiz miktar (en az 10 ₺)",
		})
		return
	}

	paymentMethod := r.Form.Get("payment_method")
	if paymentMethod == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Ödeme yöntemi seçilmedi",
		})
		return
	}

	if strings.HasPrefix(paymentMethod, "card_") {
		cardIDStr := strings.TrimPrefix(paymentMethod, "card_")
		cardID, err := strconv.Atoi(cardIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Geçersiz kart ID'si",
			})
			return
		}

		isOwner, err := m.DB.IsPaymentMethodOwner(cardID, userID)
		if err != nil || !isOwner {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Bu karta erişim yetkiniz yok",
			})
			return
		}
	}

	user, err := m.DB.GetUserByID(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Kullanıcı bilgileri alınamadı",
		})
		return
	}

	newBalance := user.Balance + amount
	err = m.DB.UpdateUserBalance(userID, newBalance)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Bakiye güncellenirken hata oluştu",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Bakiye başarıyla eklendi",
		"new_balance": newBalance,
	})
}
