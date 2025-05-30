package dbrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/malisalan/sideproject/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func (m *mysqlDBRepo) AllUsers() bool {
	return true
}

func (m *mysqlDBRepo) InsertReservation(res models.Reservations) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := "INSERT INTO reservations (first_name, last_name, email, phone, start_date, end_date, room_id, created_at, updated_at, reservation_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	result, err := m.DB.ExecContext(ctx, stmt, res.FirstName, res.LastName, res.Email, res.Phone, res.StartDate, res.EndDate, res.RoomID, time.Now(), time.Now(), "pending")
	if err != nil {
		return 0, err
	}

	newID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(newID), nil
}

func (m *mysqlDBRepo) InsertRoomRestrictions(r models.RoomRestrictions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := "INSERT INTO room_restrictions (start_date, end_date, room_id, reservation_id, created_at, updated_at,restriction_id) VALUES (?, ?, ?, ?, ?, ?,?)"
	_, err := m.DB.ExecContext(ctx, stmt, r.StartDate, r.EndDate, r.RoomID, r.ReservationID, time.Now(), time.Now(), r.RestrictionID)

	if err != nil {
		return err
	}
	return nil
}

func (m *mysqlDBRepo) SearchAvailabilityByDatesByRoomID(start, end time.Time, roomID int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var numRows int
	query := `SELECT count(id) FROM room_restrictions WHERE room_id = ? AND ? < end_date AND ? > start_date;`

	row := m.DB.QueryRowContext(ctx, query, roomID, start, end)
	err := row.Scan(&numRows)
	if err != nil {
		return false, err
	}
	if numRows == 0 {
		return true, nil
	}

	return false, nil
}

func (m *mysqlDBRepo) SearchAvailabilityForAllRooms(start, end time.Time) ([]models.Rooms, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var rooms []models.Rooms

	query := `
		SELECT
			r.id, r.room_name
		FROM
			rooms r
		WHERE
			r.id NOT IN (
				SELECT rr.room_id
				FROM room_restrictions rr
				WHERE (? <= rr.end_date AND ? >= rr.start_date)
			)
	`

	rows, err := m.DB.QueryContext(ctx, query, start, end)
	if err != nil {
		return rooms, err
	}
	for rows.Next() {
		var room models.Rooms
		err := rows.Scan(&room.ID, &room.RoomName)
		if err != nil {
			return rooms, err
		}
		rooms = append(rooms, room)
	}
	if err = rows.Err(); err != nil {
		return rooms, err
	}
	return rooms, nil
}

func (m *mysqlDBRepo) GetRoomByID(id int) (models.Rooms, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var rooms models.Rooms
	var createdAt, updatedAt string

	query := `SELECT id, room_name, created_at, updated_at FROM rooms WHERE id = ?`

	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(&rooms.ID, &rooms.RoomName, &createdAt, &updatedAt)
	if err != nil {
		return rooms, err
	}

	rooms.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return rooms, err
	}

	rooms.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		return rooms, err
	}

	return rooms, nil
}

func (m *mysqlDBRepo) GetUserByID(id int) (models.Users, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var user models.Users
	var createdAt, updatedAt string

	query := `SELECT id, first_name, last_name, email, access_level, acc_act_status, balance, created_at, updated_at FROM users WHERE id = ?`

	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(&user.ID, &user.Firstname, &user.LastName, &user.Email, &user.Accsesslevel, &user.AccActStatus, &user.Balance, &createdAt, &updatedAt)
	if err != nil {
		return user, err
	}

	user.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return user, err
	}

	user.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		return user, err
	}

	return user, nil
}

func (m *mysqlDBRepo) UpdateUser(u models.Users) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE users SET first_name = ?, last_name = ?, email = ?, phone = ?, access_level = ?, updated_at = ? WHERE id = ?`

	_, err := m.DB.ExecContext(ctx, stmt, u.Firstname, u.LastName, u.Email, u.Phone, u.Accsesslevel, time.Now(), u.ID)
	if err != nil {
		return err
	}
	return nil
}

func (m *mysqlDBRepo) UpdateUserBalance(userID int, newBalance int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE users SET balance = ?, updated_at = ? WHERE id = ?`

	_, err := m.DB.ExecContext(ctx, stmt, newBalance, time.Now(), userID)
	if err != nil {
		return err
	}
	return nil
}

func (m *mysqlDBRepo) Authenticate(email, testpassword string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var id int
	var hashedPassword string

	row := m.DB.QueryRowContext(ctx, "SELECT id, password FROM users WHERE email = ?", email)

	err := row.Scan(&id, &hashedPassword)
	if err != nil {
		return id, "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(testpassword))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return 0, "", errors.New("Geçersiz parola")
	} else if err != nil {
		return 0, "", err
	}
	return id, hashedPassword, nil
}

func (m *mysqlDBRepo) InsertUser(u models.Users) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	hashedPassword := u.Password

	if len(u.Password) > 0 && !strings.HasPrefix(u.Password, "$2a$") {

		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			return 0, err
		}
		hashedPassword = string(hash)
	}

	stmt := `INSERT INTO users (first_name, last_name, email, password, access_level, acc_act_status, activation_token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := m.DB.ExecContext(ctx, stmt, u.Firstname, u.LastName, u.Email, hashedPassword, u.Accsesslevel, u.AccActStatus, u.ActivationToken, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}

	newID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(newID), nil
}

func (m *mysqlDBRepo) GetReservationsByUserID(userID int) ([]models.Reservations, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var reservations []models.Reservations

	query := `
		SELECT r.id, r.first_name, r.last_name, r.email, r.phone, 
			r.start_date, r.end_date, r.room_id, r.created_at, r.updated_at,
			r.reservation_status,
			rm.id, rm.room_name
		FROM reservations r
		LEFT JOIN rooms rm ON (r.room_id = rm.id)
		JOIN users u ON (r.email = u.email)
		WHERE u.id = ?
		ORDER BY r.start_date DESC
	`

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return reservations, err
	}
	defer rows.Close()

	for rows.Next() {
		var i models.Reservations
		var roomID int
		var createdAt, updatedAt, startDate, endDate string
		var roomCreatedAt, roomUpdatedAt string
		var reservationStatus sql.NullString

		err := rows.Scan(
			&i.ID, &i.FirstName, &i.LastName, &i.Email, &i.Phone,
			&startDate, &endDate, &roomID, &createdAt, &updatedAt,
			&reservationStatus,
			&i.Room.ID, &i.Room.RoomName,
		)
		if err != nil {
			return reservations, err
		}

		i.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		i.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		i.StartDate, _ = time.Parse("2006-01-02 15:04:05", startDate)
		i.EndDate, _ = time.Parse("2006-01-02 15:04:05", endDate)
		i.RoomID = roomID

		if reservationStatus.Valid {
			i.ReservationStatus = reservationStatus.String
		} else {
			i.ReservationStatus = "pending"
		}

		i.Room.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", roomCreatedAt)
		i.Room.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", roomUpdatedAt)

		reservations = append(reservations, i)
	}

	if err = rows.Err(); err != nil {
		return reservations, err
	}

	return reservations, nil
}

func (m *mysqlDBRepo) GetPaymentMethodsByUserID(userID int) ([]models.PaymentMethod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var paymentMethods []models.PaymentMethod

	query := `SELECT id, user_id, card_name, last_four, expiry_month, expiry_year, card_type, created_at, updated_at FROM payment_methods WHERE user_id = ?`

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return paymentMethods, err
	}
	defer rows.Close()

	for rows.Next() {
		var pm models.PaymentMethod
		var createdAt, updatedAt string

		err := rows.Scan(
			&pm.ID, &pm.UserID, &pm.CardName, &pm.LastFour,
			&pm.ExpiryMonth, &pm.ExpiryYear, &pm.CardType,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return paymentMethods, err
		}

		pm.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		pm.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		paymentMethods = append(paymentMethods, pm)
	}

	if err = rows.Err(); err != nil {
		return paymentMethods, err
	}

	return paymentMethods, nil
}

func (m *mysqlDBRepo) UpdateUserPassword(u models.Users) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE users SET password = ?, updated_at = ? WHERE id = ?`

	_, err := m.DB.ExecContext(ctx, stmt, u.Password, time.Now(), u.ID)
	if err != nil {
		return err
	}
	return nil
}

func (m *mysqlDBRepo) AddPaymentMethod(pm models.PaymentMethod) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `INSERT INTO payment_methods (user_id, card_name, last_four, expiry_month, expiry_year, card_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := m.DB.ExecContext(ctx, stmt,
		pm.UserID, pm.CardName, pm.LastFour, pm.ExpiryMonth,
		pm.ExpiryYear, pm.CardType, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (m *mysqlDBRepo) UpdatePaymentMethod(pm models.PaymentMethod) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE payment_methods SET card_name = ?, expiry_month = ?, expiry_year = ?, updated_at = ? WHERE id = ? AND user_id = ?`

	_, err := m.DB.ExecContext(ctx, stmt,
		pm.CardName, pm.ExpiryMonth, pm.ExpiryYear,
		time.Now(), pm.ID, pm.UserID)
	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) GetPaymentMethodByID(id int) (models.PaymentMethod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var pm models.PaymentMethod
	var createdAt, updatedAt string

	query := `SELECT id, user_id, card_name, last_four, expiry_month, expiry_year, card_type, created_at, updated_at FROM payment_methods WHERE id = ?`

	row := m.DB.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&pm.ID, &pm.UserID, &pm.CardName, &pm.LastFour,
		&pm.ExpiryMonth, &pm.ExpiryYear, &pm.CardType,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return pm, err
	}

	pm.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	pm.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return pm, nil
}

func (m *mysqlDBRepo) DeletePaymentMethod(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `DELETE FROM payment_methods WHERE id = ?`

	_, err := m.DB.ExecContext(ctx, stmt, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) IsPaymentMethodOwner(paymentID, userID int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var count int
	query := `SELECT COUNT(id) FROM payment_methods WHERE id = ? AND user_id = ?`

	row := m.DB.QueryRowContext(ctx, query, paymentID, userID)
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (m *mysqlDBRepo) IsReservationOwner(reservationID, userID int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var count int
	query := `SELECT COUNT(r.id) FROM reservations r JOIN users u ON r.email = u.email WHERE r.id = ? AND u.id = ?`

	row := m.DB.QueryRowContext(ctx, query, reservationID, userID)
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (m *mysqlDBRepo) CancelReservation(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt1 := `DELETE FROM room_restrictions WHERE reservation_id = ?`
	_, err := m.DB.ExecContext(ctx, stmt1, id)
	if err != nil {
		return err
	}

	stmt2 := `DELETE FROM reservations WHERE id = ?`
	_, err = m.DB.ExecContext(ctx, stmt2, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) UpdateReservationStatus(id int, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE reservations SET reservation_status = ?, updated_at = ? WHERE id = ?`
	_, err := m.DB.ExecContext(ctx, stmt, status, time.Now(), id)
	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) GetAllReservations() ([]models.Reservations, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var reservations []models.Reservations

	query := `
		SELECT r.id, r.first_name, r.last_name, r.email, r.phone, 
			r.start_date, r.end_date, r.room_id, r.created_at, r.updated_at,
			r.reservation_status,
			rm.id, rm.room_name, rm.created_at, rm.updated_at
		FROM reservations r
		LEFT JOIN rooms rm ON (r.room_id = rm.id)
		ORDER BY r.id DESC
	`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return reservations, err
	}
	defer rows.Close()

	for rows.Next() {
		var i models.Reservations
		var roomID int
		var createdAt, updatedAt, startDate, endDate string
		var roomCreatedAt, roomUpdatedAt string
		var reservationStatus sql.NullString

		err := rows.Scan(
			&i.ID, &i.FirstName, &i.LastName, &i.Email, &i.Phone,
			&startDate, &endDate, &roomID, &createdAt, &updatedAt,
			&reservationStatus,
			&i.Room.ID, &i.Room.RoomName, &roomCreatedAt, &roomUpdatedAt,
		)
		if err != nil {
			return reservations, err
		}

		i.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		i.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		i.StartDate, _ = time.Parse("2006-01-02 15:04:05", startDate)
		i.EndDate, _ = time.Parse("2006-01-02 15:04:05", endDate)
		i.RoomID = roomID

		if reservationStatus.Valid {
			i.ReservationStatus = reservationStatus.String
		} else {
			i.ReservationStatus = "pending"
		}

		i.Room.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", roomCreatedAt)
		i.Room.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", roomUpdatedAt)

		reservations = append(reservations, i)
	}

	if err = rows.Err(); err != nil {
		return reservations, err
	}

	return reservations, nil
}

func (m *mysqlDBRepo) GetAllRooms() ([]models.Rooms, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var rooms []models.Rooms

	query := `SELECT id, room_name, created_at, updated_at FROM rooms ORDER BY room_name`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return rooms, err
	}
	defer rows.Close()

	for rows.Next() {
		var r models.Rooms
		var createdAt, updatedAt string
		err := rows.Scan(
			&r.ID,
			&r.RoomName,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return rooms, err
		}
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		r.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		rooms = append(rooms, r)
	}

	if err = rows.Err(); err != nil {
		return rooms, err
	}

	return rooms, nil
}

func (m *mysqlDBRepo) InsertRoom(room models.Rooms) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	INSERT INTO rooms (room_name, created_at, updated_at)
	VALUES (?, ?, ?)
	`

	now := time.Now()
	result, err := m.DB.ExecContext(ctx, query, room.RoomName, now, now)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (m *mysqlDBRepo) UpdateRoom(room models.Rooms) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	UPDATE rooms SET room_name = ?, updated_at = ?
	WHERE id = ?
	`

	_, err := m.DB.ExecContext(ctx, query, room.RoomName, time.Now(), room.ID)
	return err
}

func (m *mysqlDBRepo) DeleteRoom(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	SELECT COUNT(*) FROM reservations
	WHERE room_id = ?
	`
	var count int
	err := m.DB.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("bu odaya ait rezervasyonlar bulunduğu için oda silinemez")
	}

	query = `DELETE FROM rooms WHERE id = ?`
	_, err = m.DB.ExecContext(ctx, query, id)
	return err
}

func (m *mysqlDBRepo) GetAllUsers() ([]models.Users, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var users []models.Users

	query := `SELECT id, first_name, last_name, email, phone, access_level, acc_act_status, created_at, updated_at FROM users ORDER BY last_name, first_name`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return users, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.Users
		var createdAt, updatedAt string
		var phone sql.NullString
		var accActStatus sql.NullString
		err := rows.Scan(
			&u.ID,
			&u.Firstname,
			&u.LastName,
			&u.Email,
			&phone,
			&u.Accsesslevel,
			&accActStatus,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return users, err
		}
		if phone.Valid {
			u.Phone = phone.String
		} else {
			u.Phone = ""
		}
		if accActStatus.Valid {
			u.AccActStatus = accActStatus.String
		} else {
			u.AccActStatus = ""
		}
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return users, err
	}

	return users, nil
}

func (m *mysqlDBRepo) DeleteUser(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var userEmail string
	query := `SELECT email FROM users WHERE id = ?`
	err := m.DB.QueryRowContext(ctx, query, id).Scan(&userEmail)
	if err != nil {
		return err
	}

	query = `SELECT COUNT(*) FROM reservations WHERE email = ?`
	var count int
	err = m.DB.QueryRowContext(ctx, query, userEmail).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("bu kullanıcıya ait rezervasyonlar bulunduğu için kullanıcı silinemez")
	}

	query = `DELETE FROM users WHERE id = ?`
	_, err = m.DB.ExecContext(ctx, query, id)
	return err
}

func (m *mysqlDBRepo) ActivateUserByToken(token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `UPDATE users SET acc_act_status = 'confirmed', activation_token = NULL WHERE activation_token = ? AND acc_act_status = 'pending'`
	res, err := m.DB.ExecContext(ctx, stmt, token)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (m *mysqlDBRepo) UpdateReservation(res models.Reservations) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE reservations
		SET first_name = ?, last_name = ?, email = ?, phone = ?, 
			start_date = ?, end_date = ?, room_id = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := m.DB.ExecContext(ctx, query,
		res.FirstName, res.LastName, res.Email, res.Phone,
		res.StartDate, res.EndDate, res.RoomID, time.Now(), res.ID)

	return err
}

func (m *mysqlDBRepo) DeleteRoomRestrictionByReservationID(reservationID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `DELETE FROM room_restrictions WHERE reservation_id = ?`

	_, err := m.DB.ExecContext(ctx, query, reservationID)

	return err
}

func (m *mysqlDBRepo) GetAllStaff() ([]models.StaffInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var staff []models.StaffInfo

	query := `SELECT id, first_name, last_name, email, phone, staff_rank, floor, bio, photo_url, 
				created_at, updated_at FROM staff_info ORDER BY last_name, first_name`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return staff, err
	}
	defer rows.Close()

	for rows.Next() {
		var s models.StaffInfo
		var createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&s.ID,
			&s.FirstName,
			&s.LastName,
			&s.Email,
			&s.Phone,
			&s.StaffRank,
			&s.Floor,
			&s.Bio,
			&s.PhotoURL,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return staff, err
		}

		if createdAt.Valid {
			s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
		}
		if updatedAt.Valid {
			s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
		}
		staff = append(staff, s)
	}

	if err = rows.Err(); err != nil {
		return staff, err
	}

	return staff, nil
}

func (m *mysqlDBRepo) GetStaffByID(id int) (models.StaffInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var staff models.StaffInfo
	var createdAt, updatedAt sql.NullString

	query := `SELECT id, first_name, last_name, email, phone, staff_rank, floor, bio, photo_url, 
				created_at, updated_at FROM staff_info WHERE id = ?`

	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&staff.ID,
		&staff.FirstName,
		&staff.LastName,
		&staff.Email,
		&staff.Phone,
		&staff.StaffRank,
		&staff.Floor,
		&staff.Bio,
		&staff.PhotoURL,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return staff, err
	}

	if createdAt.Valid {
		staff.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
	}
	if updatedAt.Valid {
		staff.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
	}

	return staff, nil
}

func (m *mysqlDBRepo) InsertStaff(staff models.StaffInfo) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	INSERT INTO staff_info (first_name, last_name, email, phone, staff_rank, floor, bio, photo_url, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := m.DB.ExecContext(ctx, query,
		staff.FirstName,
		staff.LastName,
		staff.Email,
		staff.Phone,
		staff.StaffRank,
		staff.Floor,
		staff.Bio,
		staff.PhotoURL,
		now,
		now)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (m *mysqlDBRepo) UpdateStaff(staff models.StaffInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	UPDATE staff_info SET first_name = ?, last_name = ?, email = ?, phone = ?, 
						 staff_rank = ?, floor = ?, bio = ?, photo_url = ?, updated_at = ?
	WHERE id = ?
	`

	_, err := m.DB.ExecContext(ctx, query,
		staff.FirstName,
		staff.LastName,
		staff.Email,
		staff.Phone,
		staff.StaffRank,
		staff.Floor,
		staff.Bio,
		staff.PhotoURL,
		time.Now(),
		staff.ID)

	return err
}

func (m *mysqlDBRepo) DeleteStaff(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `DELETE FROM staff_info WHERE id = ?`
	_, err := m.DB.ExecContext(ctx, query, id)
	return err
}

func (m *mysqlDBRepo) GetRoomInfoByRoomID(roomID int) (models.RoomInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var roomInfo models.RoomInfo
	var createdAt, updatedAt string

	query := `
		SELECT id, rooms_id, first_pic_url, first_text, 
		first_tittle_fontawesome, first_tittle, first_tittle_text,
		second_tittle_fontawesome, second_tittle, second_tittle_text,
		third_tittle_fontawesome, third_tittle, third_tittle_text,
		room_max_cap, room_daily_price, created_at, updated_at
		FROM room_info WHERE rooms_id = ?
	`

	row := m.DB.QueryRowContext(ctx, query, roomID)
	err := row.Scan(
		&roomInfo.ID,
		&roomInfo.RoomsID,
		&roomInfo.FirstPicURL,
		&roomInfo.FirstText,
		&roomInfo.FirstTittleFontawesome,
		&roomInfo.FirstTittle,
		&roomInfo.FirstTittleText,
		&roomInfo.SecondTittleFontawesome,
		&roomInfo.SecondTittle,
		&roomInfo.SecondTittleText,
		&roomInfo.ThirdTittleFontawesome,
		&roomInfo.ThirdTittle,
		&roomInfo.ThirdTittleText,
		&roomInfo.RoomMaxCap,
		&roomInfo.RoomDailyPrice,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return roomInfo, err
	}

	roomInfo.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		m.App.ErrorLog.Printf("CreatedAt dönüştürme hatası: %v", err)
		roomInfo.CreatedAt = time.Now()
	}

	roomInfo.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		m.App.ErrorLog.Printf("UpdatedAt dönüştürme hatası: %v", err)
		roomInfo.UpdatedAt = time.Now()
	}

	room, err := m.GetRoomByID(roomID)
	if err != nil {
		return roomInfo, err
	}
	roomInfo.Room = room

	return roomInfo, nil
}

func (m *mysqlDBRepo) InsertRoomInfo(roomInfo models.RoomInfo) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO room_info (
			rooms_id, first_pic_url, first_text, 
			first_tittle_fontawesome, first_tittle, first_tittle_text,
			second_tittle_fontawesome, second_tittle, second_tittle_text,
			third_tittle_fontawesome, third_tittle, third_tittle_text,
			room_max_cap, room_daily_price, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := m.DB.ExecContext(ctx, query,
		roomInfo.RoomsID,
		roomInfo.FirstPicURL,
		roomInfo.FirstText,
		roomInfo.FirstTittleFontawesome,
		roomInfo.FirstTittle,
		roomInfo.FirstTittleText,
		roomInfo.SecondTittleFontawesome,
		roomInfo.SecondTittle,
		roomInfo.SecondTittleText,
		roomInfo.ThirdTittleFontawesome,
		roomInfo.ThirdTittle,
		roomInfo.ThirdTittleText,
		roomInfo.RoomMaxCap,
		roomInfo.RoomDailyPrice,
		now,
		now,
	)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (m *mysqlDBRepo) UpdateRoomInfo(roomInfo models.RoomInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE room_info SET
			rooms_id = ?,
			first_pic_url = ?,
			first_text = ?,
			first_tittle_fontawesome = ?,
			first_tittle = ?,
			first_tittle_text = ?,
			second_tittle_fontawesome = ?,
			second_tittle = ?,
			second_tittle_text = ?,
			third_tittle_fontawesome = ?,
			third_tittle = ?,
			third_tittle_text = ?,
			room_max_cap = ?,
			room_daily_price = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err := m.DB.ExecContext(ctx, query,
		roomInfo.RoomsID,
		roomInfo.FirstPicURL,
		roomInfo.FirstText,
		roomInfo.FirstTittleFontawesome,
		roomInfo.FirstTittle,
		roomInfo.FirstTittleText,
		roomInfo.SecondTittleFontawesome,
		roomInfo.SecondTittle,
		roomInfo.SecondTittleText,
		roomInfo.ThirdTittleFontawesome,
		roomInfo.ThirdTittle,
		roomInfo.ThirdTittleText,
		roomInfo.RoomMaxCap,
		roomInfo.RoomDailyPrice,
		time.Now(),
		roomInfo.ID,
	)

	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) DeleteRoomInfo(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "DELETE FROM room_info WHERE id = ?"
	_, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *mysqlDBRepo) InsertReservationPayStatus(payStatus models.ReservationPayStatus) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO reservation_pay_status (
			reservation_id, total_amount, previously_paid, payment_status, payment_method, 
			payment_date, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := m.DB.ExecContext(ctx, query,
		payStatus.ReservationID,
		payStatus.TotalAmount,
		payStatus.PreviouslyPaid,
		payStatus.PaymentStatus,
		payStatus.PaymentMethod,
		payStatus.PaymentDate,
		now,
		now,
	)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (m *mysqlDBRepo) GetReservationPayStatusByReservationID(reservationID int) (models.ReservationPayStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var payStatus models.ReservationPayStatus
	var paymentDate, createdAt, updatedAt *string

	query := `
		SELECT id, reservation_id, total_amount, previously_paid, payment_status, payment_method,
		payment_date, created_at, updated_at
		FROM reservation_pay_status WHERE reservation_id = ?
	`

	row := m.DB.QueryRowContext(ctx, query, reservationID)
	err := row.Scan(
		&payStatus.ID,
		&payStatus.ReservationID,
		&payStatus.TotalAmount,
		&payStatus.PreviouslyPaid,
		&payStatus.PaymentStatus,
		&payStatus.PaymentMethod,
		&paymentDate,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return payStatus, err
	}

	if paymentDate != nil {
		pd, err := time.Parse("2006-01-02 15:04:05", *paymentDate)
		if err == nil {
			payStatus.PaymentDate = &pd
		}
	}

	if createdAt != nil {
		payStatus.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", *createdAt)
	}

	if updatedAt != nil {
		payStatus.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", *updatedAt)
	}

	return payStatus, nil
}

func (m *mysqlDBRepo) UpdateReservationPayStatus(payStatus models.ReservationPayStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE reservation_pay_status SET
			total_amount = ?,
			previously_paid = ?,
			payment_status = ?,
			payment_method = ?,
			payment_date = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err := m.DB.ExecContext(ctx, query,
		payStatus.TotalAmount,
		payStatus.PreviouslyPaid,
		payStatus.PaymentStatus,
		payStatus.PaymentMethod,
		payStatus.PaymentDate,
		time.Now(),
		payStatus.ID,
	)

	return err
}
