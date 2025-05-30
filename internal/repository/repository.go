package repository

import (
	"time"

	"github.com/malisalan/sideproject/internal/models"
)

type DatabaseRepo interface {
	AllUsers() bool
	InsertReservation(res models.Reservations) (int, error)
	InsertRoomRestrictions(r models.RoomRestrictions) error
	SearchAvailabilityByDatesByRoomID(start, end time.Time, roomID int) (bool, error)
	SearchAvailabilityForAllRooms(start, end time.Time) ([]models.Rooms, error)
	GetRoomByID(id int) (models.Rooms, error)
	GetUserByID(id int) (models.Users, error)
	UpdateUser(u models.Users) error
	UpdateUserBalance(userID int, newBalance int) error
	Authenticate(email, testPassword string) (int, string, error)
	InsertUser(u models.Users) (int, error)
	GetReservationsByUserID(userID int) ([]models.Reservations, error)
	GetPaymentMethodsByUserID(userID int) ([]models.PaymentMethod, error)
	UpdateUserPassword(u models.Users) error
	AddPaymentMethod(pm models.PaymentMethod) (int, error)
	DeletePaymentMethod(id int) error
	IsPaymentMethodOwner(paymentID, userID int) (bool, error)
	IsReservationOwner(reservationID, userID int) (bool, error)
	CancelReservation(id int) error
	UpdatePaymentMethod(pm models.PaymentMethod) error
	GetPaymentMethodByID(id int) (models.PaymentMethod, error)
	GetAllReservations() ([]models.Reservations, error)
	UpdateReservation(res models.Reservations) error
	DeleteRoomRestrictionByReservationID(reservationID int) error
	UpdateReservationStatus(id int, status string) error

	GetAllRooms() ([]models.Rooms, error)
	InsertRoom(room models.Rooms) (int, error)
	UpdateRoom(room models.Rooms) error
	DeleteRoom(id int) error
	GetAllUsers() ([]models.Users, error)
	DeleteUser(id int) error
	ActivateUserByToken(token string) (bool, error)

	GetAllStaff() ([]models.StaffInfo, error)
	GetStaffByID(id int) (models.StaffInfo, error)
	InsertStaff(staff models.StaffInfo) (int, error)
	UpdateStaff(staff models.StaffInfo) error
	DeleteStaff(id int) error

	GetRoomInfoByRoomID(roomID int) (models.RoomInfo, error)
	InsertRoomInfo(roomInfo models.RoomInfo) (int, error)
	UpdateRoomInfo(roomInfo models.RoomInfo) error
	DeleteRoomInfo(id int) error

	// Payment Status functions
	InsertReservationPayStatus(payStatus models.ReservationPayStatus) (int, error)
	GetReservationPayStatusByReservationID(reservationID int) (models.ReservationPayStatus, error)
	UpdateReservationPayStatus(payStatus models.ReservationPayStatus) error
}
