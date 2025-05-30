package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/malisalan/sideproject/internal/config"
	"github.com/malisalan/sideproject/internal/driver"
	"github.com/malisalan/sideproject/internal/handlers"
	"github.com/malisalan/sideproject/internal/helpers"
	"github.com/malisalan/sideproject/internal/models"
	"github.com/malisalan/sideproject/internal/render"
)

const portnumber = "8080"

var app config.AppConfig
var session *scs.SessionManager
var infoLog *log.Logger
var errorLog *log.Logger

func main() {
	db, err := run()
	if err != nil {
		log.Fatal(err)
	}
	defer db.SQL.Close()
	defer close(app.MailChan)
	fmt.Println("Mail sunucusu başlatılıyor...")
	time.Sleep(2 * time.Second)
	fmt.Println("Mail sunucusu başlatıldı.")
	ListenForMail()

	fmt.Println("Server is starting...")
	time.Sleep(2 * time.Second)
	fmt.Printf("Server is started. Please open your browser and go to http://localhost:%s\n", portnumber)
	srv := &http.Server{
		Addr:    ":" + portnumber,
		Handler: routes(&app),
	}
	err = srv.ListenAndServe()
	log.Fatal(err)
	time.Sleep(3 * time.Second)
}

func run() (*driver.DB, error) {

	gob.Register(models.Reservation{})
	gob.Register(models.Users{})
	gob.Register(models.Rooms{})
	gob.Register(models.Restrictions{})
	gob.Register(models.PaymentMethod{})
	gob.Register(models.Reservations{})

	mailChan := make(chan models.MailData)
	app.MailChan = mailChan

	app.InProduction = false

	infoLog = log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	app.InfoLog = infoLog

	errorLog = log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	app.ErrorLog = errorLog

	session = scs.New()
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = app.InProduction

	app.Session = session

	log.Println("Databe bağlantısı kuruluyor...")
	time.Sleep(2 * time.Second)
	db, err := driver.ConnectSQL("SQL KOMUTU")
	if err != nil {
		log.Fatal("Database bağlanırken hata:", err)
	} else if db != nil {
		log.Println("Database bağlantısı başarılı.")
	}

	tc, err := render.CreateTemplateCache()
	if err != nil {
		log.Fatal("Error creating template cache:", err)
		return nil, err
	}
	app.TemplateCache = tc
	app.UseCache = false

	repo := handlers.NewRepo(&app, db)
	handlers.NewHandlers(repo)
	helpers.NewHelpers(&app)
	render.NewRenderer(&app)
	render.NewRepo(repo.DB)
	return db, nil
}
