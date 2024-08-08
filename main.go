package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"todo-go/common"
	"todo-go/directory"
	"todo-go/server"
	"todo-go/store"

	"github.com/aserto-dev/go-aserto"
	"github.com/aserto-dev/go-aserto/ds/v3"
	"github.com/aserto-dev/go-aserto/middleware"
	"github.com/aserto-dev/go-aserto/middleware/gorillaz"
	"github.com/aserto-dev/go-authorizer/aserto/authorizer/v2"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"google.golang.org/grpc"
)

func main() {
	initLogger()
	options := loadOptions()

	// Initialize the Todo Store
	db, dbError := store.NewStore()
	if dbError != nil {
		log.Fatal("Failed to create store:", dbError)
	}

	// Create a directory client
	dir, err := directory.NewDirectory(options.directory)
	if err != nil {
		log.Fatalln("Failed to create directory connection:", err)
	}

	// Initialize the Server
	srv := server.Server{Store: db, Directory: dir}

	// Create an authorizer client
	authorizerClient, err := NewAuthorizerClient(options.authorizer)
	if err != nil {
		log.Fatalln("Retry: Failed to create authorizer client:", err)
	}

	router := mux.NewRouter()

	// Add JWT validation. This middleware validates incoming JWT tokens and stores the subject name in the request
	// context.
	router.Use(JWTValidator(options.jwksKeysURL))

	// Create authorization middleware
	mw := NewAuthorizationMiddleware(authorizerClient,
		&middleware.Policy{
			Name:     options.policyInstanceName,
			Decision: "allowed",
			Root:     options.policyRoot,
		},
		options.policyRoot,
	).WithResourceMapper(srv.TodoOwnerResourceMapper)

	// Set up routes
	router.Handle("/users/{userID}", mw.HandlerFunc(dir.GetUser)).Methods("GET")

	router.Handle("/todos", mw.HandlerFunc(srv.GetTodos)).Methods("GET")
	router.Handle("/todos/{id}", mw.HandlerFunc(srv.UpdateTodo)).Methods("PUT")
	router.Handle("/todos/{id}", mw.HandlerFunc(srv.DeleteTodo)).Methods("DELETE")

	router.Handle(
		"/todos",
		mw.Check(
			gorillaz.WithObjectType("resource-creator"),
			gorillaz.WithRelation("member"),
			gorillaz.WithObjectID("resource-creators"),
			gorillaz.WithPolicyPath("rebac.check"),
		).HandlerFunc(srv.InsertTodo)).Methods("POST")

	srv.Start(router)
}

type options struct {
	authorizer *aserto.Config
	directory  *ds.Config

	policyInstanceName string
	policyRoot         string

	jwksKeysURL string
}

func initLogger() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func loadOptions() *options {
	if envFileError := godotenv.Load(); envFileError != nil {
		log.Fatal("Error loading .env file")
	}

	authorizerAddr := os.Getenv("ASERTO_AUTHORIZER_SERVICE_URL")
	if authorizerAddr == "" {
		authorizerAddr = "authorizer.prod.aserto.com:8443"
	}

	directoryAddr := os.Getenv("ASERTO_DIRECTORY_SERVICE_URL")
	if directoryAddr == "" {
		directoryAddr = "directory.prod.aserto.com:8443"
	}

	log.Printf("Authorizer: %s\n", authorizerAddr)
	log.Printf("Directory:  %s\n", directoryAddr)

	return &options{
		authorizer: &aserto.Config{
			Address:    authorizerAddr,
			APIKey:     os.Getenv("ASERTO_AUTHORIZER_API_KEY"),
			CACertPath: os.ExpandEnv(getEnv("ASERTO_AUTHORIZER_GRPC_CA_CERT_PATH", "ASERTO_GRPC_CA_CERT_PATH")),
			TenantID:   os.Getenv("ASERTO_TENANT_ID"),
		},
		directory: &ds.Config{
			Config: &aserto.Config{
				Address:    directoryAddr,
				APIKey:     os.Getenv("ASERTO_DIRECTORY_API_KEY"),
				CACertPath: os.ExpandEnv(getEnv("ASERTO_DIRECTORY_GRPC_CA_CERT_PATH", "ASERTO_GRPC_CA_CERT_PATH")),
				TenantID:   os.Getenv("ASERTO_TENANT_ID"),
			}},
		jwksKeysURL:        os.Getenv("JWKS_URI"),
		policyInstanceName: os.Getenv("ASERTO_POLICY_INSTANCE_NAME"),
		policyRoot:         os.Getenv("ASERTO_POLICY_ROOT"),
	}
}

func NewAuthorizerClient(cfg *aserto.Config) (authorizer.AuthorizerClient, error) {
	conn, err := newConnection(cfg)
	if err != nil {
		return nil, err
	}

	return authorizer.NewAuthorizerClient(conn), nil
}

func newConnection(cfg *aserto.Config) (grpc.ClientConnInterface, error) {
	connectionOpts, err := cfg.ToConnectionOptions(aserto.NewDialOptionsProvider())
	if err != nil {
		return nil, err
	}

	conn, err := aserto.NewConnection(connectionOpts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func NewAuthorizationMiddleware(authClient authorizer.AuthorizerClient, policy *middleware.Policy, policyRoot string) *gorillaz.Middleware {
	mw := gorillaz.New(authClient, policy).WithPolicyFromURL(policyRoot)
	// Retrieve the caller's identity from the context value set by the JWTValidator middleware
	mw.Identity.Subject().FromContextValue(common.ContextKeySubject)
	return mw
}

func JWTValidator(jwksKeysURL string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keys, err := jwk.Fetch(r.Context(), jwksKeysURL)
			if err != nil || keys == nil {
				log.Printf("Failed to fetch JWKs from [%s]: %s", jwksKeysURL, err)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			authorizationHeader := r.Header.Get("Authorization")
			tokenBytes := []byte(strings.Replace(authorizationHeader, "Bearer ", "", 1))

			jwt.WithVerifyAuto(nil)
			token, err := jwt.Parse(tokenBytes, jwt.WithKeySet(keys))
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), common.ContextKeySubject, token.Subject())))
		})
	}
}

func getEnv(vars ...string) string {
	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			return val
		}
	}
	return ""
}
