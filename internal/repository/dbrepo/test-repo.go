package dbrepo

import (
	"errors"
	"time"

	"github.com/malisalan/sideproject/internal/models"
)

func (m *testDBRepo) AllUsers() bool {
	return true
}

func (m *testDBRepo) InsertReservation(res models.Reservations) (int, error) {

	return 1, nil
}

func (m *testDBRepo) InsertRoomRestrictions(r models.RoomRestrictions) error {
	return nil
}

func (m *testDBRepo) SearchAvailabilityByDatesByRoomID(start, end time.Time, roomID int) (bool, error) {

	return false, nil
}

func (m *testDBRepo) SearchAvailabilityForAllRooms(start, end time.Time) ([]models.Rooms, error) {
	var rooms []models.Rooms
	return rooms, nil
}

func (m *testDBRepo) GetRoomByID(id int) (models.Rooms, error) {
	var rooms models.Rooms
	if id > 2 {
		return rooms, errors.New("room not found")
	}
	return rooms, nil
}

func (m *testDBRepo) GetUserByID(id int) (models.Users, error) {
	var user models.Users
	if id > 2 {
		return user, errors.New("user not found")
	}

	user = models.Users{
		ID:           id,
		Firstname:    "Test",
		LastName:     "User",
		Email:        "test@example.com",
		Phone:        "555-1234",
		Password:     "password",
		Accsesslevel: 1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return user, nil
}

func (m *testDBRepo) UpdateUser(u models.Users) error {
	return nil
}

func (m *testDBRepo) Authenticate(email, password string) (int, string, error) {
	return 1, "", nil
}
func (m *testDBRepo) InsertUser(u models.Users) (int, error) {
	return 1, nil
}

func (m *testDBRepo) DeletePaymentMethod(id int) error {
	return nil
}

func (m *testDBRepo) IsPaymentMethodOwner(paymentID, userID int) (bool, error) {
	return true, nil
}

func (m *testDBRepo) IsReservationOwner(reservationID, userID int) (bool, error) {
	return true, nil
}

func (m *testDBRepo) CancelReservation(id int) error {
	return nil
}

func (m *testDBRepo) UpdateReservationStatus(id int, status string) error {
	return nil
}

func (m *testDBRepo) GetReservationsByUserID(userID int) ([]models.Reservations, error) {
	var reservations []models.Reservations

	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, 3)

	res1 := models.Reservations{
		ID:        1,
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Phone:     "555-1234",
		StartDate: startDate,
		EndDate:   endDate,
		RoomID:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Room: models.Rooms{
			ID:       1,
			RoomName: "General's Quarters",
		},
	}

	res2 := models.Reservations{
		ID:        2,
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Phone:     "555-1234",
		StartDate: startDate.AddDate(0, 1, 0),
		EndDate:   endDate.AddDate(0, 1, 0),
		RoomID:    2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Room: models.Rooms{
			ID:       2,
			RoomName: "Major's Suite",
		},
	}

	reservations = append(reservations, res1, res2)
	return reservations, nil
}

func (m *testDBRepo) GetPaymentMethodsByUserID(userID int) ([]models.PaymentMethod, error) {
	var paymentMethods []models.PaymentMethod

	pm1 := models.PaymentMethod{
		ID:          1,
		UserID:      userID,
		CardName:    "Test User",
		LastFour:    "1234",
		ExpiryMonth: 12,
		ExpiryYear:  2025,
		CardType:    "Visa",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm2 := models.PaymentMethod{
		ID:          2,
		UserID:      userID,
		CardName:    "Test User",
		LastFour:    "5678",
		ExpiryMonth: 6,
		ExpiryYear:  2024,
		CardType:    "MasterCard",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	paymentMethods = append(paymentMethods, pm1, pm2)
	return paymentMethods, nil
}

func (m *testDBRepo) UpdateUserPassword(u models.Users) error {
	return nil
}

func (m *testDBRepo) AddPaymentMethod(pm models.PaymentMethod) (int, error) {
	return 1, nil
}

func (m *testDBRepo) UpdatePaymentMethod(pm models.PaymentMethod) error {
	return nil
}

func (m *testDBRepo) GetPaymentMethodByID(id int) (models.PaymentMethod, error) {
	var pm models.PaymentMethod

	if id > 0 {
		pm = models.PaymentMethod{
			ID:          id,
			UserID:      1,
			CardName:    "Test User",
			LastFour:    "1234",
			ExpiryMonth: 12,
			ExpiryYear:  2025,
			CardType:    "Visa",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		return pm, nil
	}

	return pm, errors.New("payment method not found")
}

func (m *testDBRepo) GetAllReservations() ([]models.Reservations, error) {
	var reservations []models.Reservations
	return reservations, nil
}

func (m *testDBRepo) GetAllRooms() ([]models.Rooms, error) {
	var rooms []models.Rooms
	return rooms, nil
}

func (m *testDBRepo) InsertRoom(room models.Rooms) (int, error) {
	return 1, nil
}

func (m *testDBRepo) UpdateRoom(room models.Rooms) error {
	return nil
}

func (m *testDBRepo) DeleteRoom(id int) error {
	return nil
}

func (m *testDBRepo) GetAllUsers() ([]models.Users, error) {
	var users []models.Users
	return users, nil
}

func (m *testDBRepo) DeleteUser(id int) error {
	return nil
}

func (m *testDBRepo) ActivateUserByToken(token string) (bool, error) {
	if token == "validtoken" {
		return true, nil
	}
	return false, nil
}

func (m *testDBRepo) UpdateReservation(res models.Reservations) error {
	return nil
}

func (m *testDBRepo) DeleteRoomRestrictionByReservationID(reservationID int) error {
	return nil
}

func (m *testDBRepo) GetAllStaff() ([]models.StaffInfo, error) {
	return []models.StaffInfo{}, nil
}

func (m *testDBRepo) GetStaffByID(id int) (models.StaffInfo, error) {
	return models.StaffInfo{}, nil
}

func (m *testDBRepo) InsertStaff(staff models.StaffInfo) (int, error) {
	return 1, nil
}

func (m *testDBRepo) UpdateStaff(staff models.StaffInfo) error {
	return nil
}

func (m *testDBRepo) DeleteStaff(id int) error {
	return nil
}

func (m *testDBRepo) GetRoomInfoByRoomID(roomID int) (models.RoomInfo, error) {
	var roomInfo models.RoomInfo
	return roomInfo, nil
}

func (m *testDBRepo) InsertRoomInfo(roomInfo models.RoomInfo) (int, error) {
	return 1, nil
}

func (m *testDBRepo) UpdateRoomInfo(roomInfo models.RoomInfo) error {
	return nil
}

func (m *testDBRepo) DeleteRoomInfo(id int) error {
	return nil
}

func (m *testDBRepo) InsertReservationPayStatus(payStatus models.ReservationPayStatus) (int, error) {
	return 1, nil
}

func (m *testDBRepo) GetReservationPayStatusByReservationID(reservationID int) (models.ReservationPayStatus, error) {
	var payStatus models.ReservationPayStatus
	return payStatus, nil
}

func (m *testDBRepo) UpdateReservationPayStatus(payStatus models.ReservationPayStatus) error {
	return nil
}

func (m *testDBRepo) UpdateUserBalance(userID int, newBalance int) error {
	return nil
}
