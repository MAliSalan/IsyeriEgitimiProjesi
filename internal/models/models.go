package models

import (
	"time"
)

type Reservation struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

type Users struct {
	ID              int
	Firstname       string
	LastName        string
	Email           string
	Phone           string
	Password        string
	Accsesslevel    int
	AccActStatus    string
	ActivationToken string
	Balance         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type User = Users

type Rooms struct {
	ID        int
	RoomName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Room = Rooms

type Restrictions struct {
	ID              int
	RestrictionName string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Reservations struct {
	ID                int
	FirstName         string
	LastName          string
	Email             string
	Phone             string
	StartDate         time.Time
	EndDate           time.Time
	RoomID            int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Room              Rooms
	ReservationStatus string
	TotalAmount       int
	PaymentStatus     string
	PaymentMethod     string
}

type RoomRestrictions struct {
	ID            int
	StartDate     time.Time
	EndDate       time.Time
	RoomID        int
	ReservationID int
	RestrictionID int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Room          Rooms
	Reservation   Reservations
	Restrictions  Restrictions
}

type PaymentMethod struct {
	ID          int
	UserID      int
	CardName    string
	LastFour    string
	ExpiryMonth int
	ExpiryYear  int
	CardType    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MailData struct {
	To      string
	From    string
	Subject string
	Content string
}

type StaffInfo struct {
	ID        int
	FirstName string
	LastName  string
	Email     string
	Phone     string
	StaffRank string
	Floor     string
	Bio       string
	PhotoURL  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoomInfo struct {
	ID                      int
	RoomsID                 int
	FirstPicURL             string
	FirstText               string
	FirstTittleFontawesome  string
	FirstTittle             string
	FirstTittleText         string
	SecondTittleFontawesome string
	SecondTittle            string
	SecondTittleText        string
	ThirdTittleFontawesome  string
	ThirdTittle             string
	ThirdTittleText         string
	RoomMaxCap              int
	RoomDailyPrice          int
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Room                    Rooms
}

type ReservationPayStatus struct {
	ID             int
	ReservationID  int
	TotalAmount    int
	PreviouslyPaid int
	PaymentStatus  string
	PaymentMethod  string
	PaymentDate    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Reservation    Reservations
}
