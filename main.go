package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo-go/server"

	"github.com/aserto-dev/go-aserto/middleware/gorillaz"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize logging
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	options, err := server.LoadOptions()
	if err != nil {
		log.Fatalln("failed to load options:", err)
	}

	// Initialize the Server
	srv, err := server.New(options)
	if err != nil {
		log.Fatalln("failed to create server:", err)
	}
	defer srv.Close()

	// Create an authorizer client
	azClient, err := NewAuthorizerClient(options.Authorizer)
	if err != nil {
		log.Fatalln("failed to create authorizer client:", err)
	}
	defer azClient.Close()

	// Create a context that is cancelled when SIGINT or SIGTERM is received
	ctx, stop := signalContext()
	defer stop()

	// This middleware validates incoming JWTs and stores the subject name in the request context.
	authn := AuthenticationMiddleware(ctx, options)

	// This middleware authorizes incoming requests.
	authz := AuthorizationMiddleware(azClient, options)

	router := AppRouter(srv, authn, authz)

	// Start the server
	go func() {
		srv.Start(router)
	}()

	// Wait for the context to be cancelled
	<-ctx.Done()

	// Gracefully shutdown the server
	srv.Shutdown(5 * time.Second)
}

func AppRouter(srv *server.Server, authn mux.MiddlewareFunc, authz *gorillaz.Middleware) *mux.Router {
	router := mux.NewRouter()

	router.Use(authn)

	// Set up routes
	router.Handle("/users/{userID}", authz.HandlerFunc(srv.GetUser)).Methods("GET")

	router.Handle("/todos", authz.HandlerFunc(srv.GetTodos)).Methods("GET")
	router.Handle("/todos/{id}", authz.HandlerFunc(srv.UpdateTodo)).Methods("PUT")
	router.Handle("/todos/{id}", authz.HandlerFunc(srv.DeleteTodo)).Methods("DELETE")

	router.Handle(
		"/todos",
		authz.Check(
			gorillaz.WithObjectType("resource-creator"),
			gorillaz.WithRelation("member"),
			gorillaz.WithObjectID("resource-creators"),
			gorillaz.WithPolicyPath("rebac.check"),
		).HandlerFunc(srv.InsertTodo)).Methods("POST")

	return router
}

// signalContext returns a context that is cancelled when SIGINT or SIGTERM is received.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
