package main

import (
	"context"
	"gopkg.in/mgo.v2"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"github.com/tutley/preach/api/handlers"
	"github.com/tutley/preach/api/helpers"
	"time"
)

var index []byte

// This will run the api server on the specified port and will serve routes at
// <hostname>:<port>/v1/

func main() {
	// Grab Configuration Variables
	err := godotenv.Load("config.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	serverPort := os.Getenv("SERVER_PORT")
	dbURL := os.Getenv("DB_URL")
	dbName := os.Getenv("DB_NAME")

	// Init the Database
	mongoMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// setup the mgo connection
			session, err := mgo.Dial("mongodb://" + dbURL)

			if err != nil {
				log.Println("DB Connect error: ", err)
				http.Error(w, "Unable to connect to database", 500)
			}

			reqSession := session.Clone()
			defer reqSession.Close()
			db := reqSession.DB(dbName)
			ctx := context.WithValue(r.Context(), helpers.DbKey, db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	helpers.SetJwtSecret([]byte(jwtSecret))

	// setup the Chi router
	r := chi.NewRouter()

	// Use Chi built-in middlewares
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount database
	r.Use(mongoMiddleware)

	// When a client closes their connection midway through a request, the
	// http.CloseNotifier will cancel the request context (ctx).
	r.Use(middleware.CloseNotify)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	// Setup routes/routers for the API. The routers are defined last in this file
	r.Post("/v1/signup", handlers.SignUpHandler)
	r.Mount("/v1/login", loginRouter())
	r.Mount("/v1", APIRouter())

	// and.... go!
	serveAddr := ":" + serverPort
	log.Println("API Server listening on: " + serverPort)
	http.ListenAndServe(serveAddr, r)
	// https://golang.org/pkg/net/http/#ListenAndServeTLS
}

// ROUTING

// LoginRouter provides the routes for loging in
func loginRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.NoCache)
	r.Use(helpers.BasicMiddleware)

	r.Get("/", handlers.SignInHandler)
	return r
}

// APIRouter handles the routes for the API
func APIRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.NoCache)
	r.Use(helpers.JwtAuthMiddleware)

	r.Get("/me", handlers.GetMeHandler)
	r.Put("/me", handlers.UpdateMeHandler)

	return r
}
