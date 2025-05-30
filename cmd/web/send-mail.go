package main

import (
	"time"

	"github.com/malisalan/sideproject/internal/models"
	mail "github.com/xhit/go-simple-mail/v2"
)

func ListenForMail() {
	go func() {
		for {
			msg := <-app.MailChan
			SendMsg(msg)
		}
	}()
}

func SendMsg(m models.MailData) {
	server := mail.NewSMTPClient()
	server.Host = "localhost"
	server.Port = 1025
	server.KeepAlive = false
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 10 * time.Second

	client, err := server.Connect()
	if err != nil {
		errorLog.Println("Mail sunucusuna bağlanırken hata:", err)
	}

	email := mail.NewMSG()
	email.SetFrom(m.From).AddTo(m.To).SetSubject(m.Subject)
	email.SetBody(mail.TextHTML, m.Content)
	err = email.Send(client)
	if err != nil {
		errorLog.Println("E-posta gönderilirken hata:", err)
	} else {
		infoLog.Println("E-posta başarıyla gönderildi.")
	}
}
