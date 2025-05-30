package main

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/malisalan/sideproject/internal/config"
	"github.com/malisalan/sideproject/internal/handlers"
)

func routes(app *config.AppConfig) http.Handler {
	mux := chi.NewRouter()

	mux.Use(middleware.Recoverer)
	mux.Use(NoSurf)
	mux.Use(SessionLoad)

	mux.Get("/", handlers.Repo.Home)
	mux.Get("/about", handlers.Repo.About)
	mux.Get("/generals-quarters", handlers.Repo.Generals)
	mux.Get("/majors-suite", handlers.Repo.Majors)

	mux.Get("/choose-room/{id}", handlers.Repo.ChooseRoom)
	mux.Get("/general-rooms", handlers.Repo.GeneralRoom)
	mux.Get("/major-rooms", handlers.Repo.MajorsRoom)
	mux.Get("/room/{id}", handlers.Repo.Room)

	mux.Get("/search-availability", handlers.Repo.Availability)
	mux.Post("/search-availability", handlers.Repo.PostAvailability)
	mux.Post("/search-availability-json", handlers.Repo.AvailabilityJSON)

	mux.Get("/contact", handlers.Repo.Contact)
	mux.Post("/contact", handlers.Repo.PostContact)

	mux.Get("/staff", handlers.Repo.Staff)
	mux.Get("/staff/{id}", handlers.Repo.StaffDetail)

	mux.Group(func(r chi.Router) {
		r.Use(Auth)
		r.Get("/make-reservation", handlers.Repo.Reservation)
		r.Post("/make-reservation", handlers.Repo.PostReservation)
	})
	mux.Get("/reservation-summary", handlers.Repo.ReservationSummary)

	mux.Get("/user/login", handlers.Repo.Login)
	mux.Post("/user/login", handlers.Repo.PostLogin)
	mux.Get("/user/logout", handlers.Repo.Logout)
	mux.Get("/user/register", handlers.Repo.Register)
	mux.Post("/user/register", handlers.Repo.PostRegister)

	mux.Group(func(r chi.Router) {
		r.Use(Auth)
		r.Get("/user/profile", handlers.Repo.Profile)
		r.Post("/user/profile", handlers.Repo.PostProfileUpdate)
		r.Get("/user/password", handlers.Repo.UserPassword)
		r.Post("/user/password", handlers.Repo.PostPasswordUpdate)
		r.Get("/user/reservations", handlers.Repo.UserReservations)
		r.Get("/user/payments", handlers.Repo.UserPayments)
		r.Post("/user/payment/add", handlers.Repo.PostPaymentAdd)
		r.Get("/user/payment/delete/{id}", handlers.Repo.DeletePaymentMethod)
		r.Post("/user/payment/edit/{id}", handlers.Repo.UpdatePaymentMethod)
		r.Get("/user/reservation/cancel/{id}", handlers.Repo.CancelReservation)
		r.Get("/api/reservation/{id}", handlers.Repo.GetReservationAPI)
		r.Get("/api/user/payment-methods", handlers.Repo.GetUserPaymentMethodsAPI)
		r.Post("/user/pay-reservation", handlers.Repo.PayReservation)
		r.Post("/user/add-balance", handlers.Repo.AddBalance)
	})

	mux.Group(func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/admin/dashboard", handlers.Repo.AdminDashboard)
		r.Get("/admin/reservations", handlers.Repo.AdminAllReservations)
		r.Get("/admin/reservations/{id}", handlers.Repo.AdminReservationDetail)
		r.Get("/admin/reservations/{id}/status/{status}", handlers.Repo.AdminUpdateReservationStatus)
		r.Post("/admin/reservations/update", handlers.Repo.AdminUpdateReservation)
		r.Get("/admin/delete-reservation/{id}", handlers.Repo.AdminDeleteReservation)
		r.Get("/admin/rooms", handlers.Repo.AdminRooms)
		r.Post("/admin/rooms/add", handlers.Repo.AdminAddRoom)
		r.Post("/admin/rooms/update", handlers.Repo.AdminUpdateRoom)
		r.Get("/admin/rooms/delete/{id}", handlers.Repo.AdminDeleteRoom)
		r.Get("/admin/rooms/{id}/roominfo", handlers.Repo.AdminEditRoomInfo)
		r.Post("/admin/rooms/{id}/roominfo", handlers.Repo.AdminUpdateRoomInfo)
		r.Get("/admin/users", handlers.Repo.AdminUsers)
		r.Post("/admin/users/update", handlers.Repo.AdminUpdateUser)
		r.Get("/admin/users/delete/{id}", handlers.Repo.AdminDeleteUser)
		r.Get("/admin/staff", handlers.Repo.AdminStaff)
		r.Post("/admin/staff/add", handlers.Repo.AdminAddStaff)
		r.Get("/admin/staff/edit/{id}", handlers.Repo.AdminEditStaff)
		r.Post("/admin/staff/update", handlers.Repo.AdminUpdateStaff)
		r.Get("/admin/staff/delete/{id}", handlers.Repo.AdminDeleteStaff)
	})
	mux.Get("/verifyaccount", handlers.Repo.VerifyAccount)
	mux.Get("/verifyaccount?act_token={token}", handlers.Repo.VerifyAccount)
	mux.Get("/test", handlers.Repo.TestPage)

	fileServer := http.FileServer(http.Dir("./static/"))

	jsAwareFileServer := JsFilesHandler(fileServer)

	mux.Handle("/static/*", http.StripPrefix("/static", jsAwareFileServer))

	mux.Handle("/admin/static/*", http.StripPrefix("/admin/static", jsAwareFileServer))

	return mux
}
