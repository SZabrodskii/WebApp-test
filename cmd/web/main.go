package main

import (
	"encoding/gob"
	"flag"
	"github.com/alexedwards/scs/v2"
	"log"
	"net/http"
	"webapp/pkg/data"
	"webapp/pkg/repository"
	"webapp/pkg/repository/dbrepo"
)

type application struct {
	DSN     string
	DB      repository.DatabaseRepo
	Session *scs.SessionManager
}

func main() {
	gob.Register(data.User{})
	app := application{}
	flag.StringVar(&app.DSN, "dsn", "host=localhost port=5433 user=postgres password=postgres dbname=users sslmode=disable timezone=UTC connect_timeout=5", "Postgres connection")
	flag.Parse()

	//connect to a db

	conn, err := app.connectToDB()
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	app.DB = &dbrepo.PostgresDBRepo{DB: conn}
	//get a session manager

	app.Session = getSession()

	log.Println("Starting server on port 8085...")
	err = http.ListenAndServe(":8085", app.routes())
	if err != nil {
		log.Fatal(err)
		return
	}
}
