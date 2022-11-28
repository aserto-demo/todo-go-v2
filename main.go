package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/aserto-dev/go-aserto/authorizer/grpc"
	"github.com/aserto-dev/go-aserto/client"
	"github.com/aserto-dev/go-directory/aserto/directory/reader/v2"

	"github.com/aserto-dev/go-aserto/middleware"
	"github.com/aserto-dev/go-aserto/middleware/http/std"
	authz "github.com/aserto-dev/go-authorizer/aserto/authorizer/v2"

	"github.com/gorilla/mux"

	"todo-go/directory"
	"todo-go/server"
	"todo-go/store"
)

func main() {
	options := loadOptions()
	ctx := context.Background()

	// Create a directory reader client
	directoryReader, err := NewDirectoryReader(ctx, options)
	if err != nil {
		log.Fatal("Failed to create directory reader client:", err)
	}

	dir := &directory.Directory{Reader: directoryReader}

	// Initialize the Todo Store
	db, dbError := store.NewStore()
	if dbError != nil {
		log.Fatal("Failed to create store:", dbError)
	}

	// Initialize the Server
	srv := server.Server{Store: db, Directory: dir}

	// Create an authorizer client
	authorizerClient, err := NewAuthorizerClient(ctx, options)
	if err != nil {
		log.Fatal("Failed to create authorizer client:", err)
	}

	// Create authorization middleware
	authorizationMiddleware := AsertoAuthorizer(authorizerClient,
		&middleware.Policy{
			Name:          options.policyInstanceName,
			Decision:      "allowed",
			InstanceLabel: options.policyInstanceLabel,
		},
		options.policyRoot,
	).WithResourceMapper(srv.TodoOwnerResourceMapper)

	router := mux.NewRouter()

	// Add JWT validation and authorization middleware
	router.Use(JWTValidator(options.jwksKeysUrl), authorizationMiddleware.Handler)

	// Set up routes
	router.HandleFunc("/todos", srv.GetTodos).Methods("GET")
	router.HandleFunc("/todos", srv.InsertTodo).Methods("POST")
	router.HandleFunc("/todos/{id}", srv.UpdateTodo).Methods("PUT")
	router.HandleFunc("/todos/{id}", srv.DeleteTodo).Methods("DELETE")
	router.HandleFunc("/users/{userID}", dir.GetUser).Methods("GET")

	srv.Start(router)
}

type options struct {
	authorizerAddr       string
	authorizerKey        string
	authorizerCACertPath string

	directoryAddr       string
	directoryKey        string
	directoryCACertPath string

	tenantID            string
	policyInstanceName  string
	policyInstanceLabel string
	policyRoot          string

	jwksKeysUrl string
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

	return &options{
		authorizerAddr:       authorizerAddr,
		authorizerKey:        os.Getenv("ASERTO_AUTHORIZER_API_KEY"),
		authorizerCACertPath: os.ExpandEnv("$ASERTO_AUTHORIZER_CERT_PATH"),
		directoryAddr:        directoryAddr,
		directoryKey:         os.Getenv("ASERTO_DIRECTORY_API_KEY"),
		directoryCACertPath:  os.ExpandEnv("$ASERTO_DIRECTORY_CERT_PATH"),
		jwksKeysUrl:          os.Getenv("JWKS_URI"),
		policyInstanceName:   os.Getenv("ASERTO_POLICY_INSTANCE_NAME"),
		policyInstanceLabel:  os.Getenv("ASERTO_POLICY_INSTANCE_LABEL"),
		policyRoot:           os.Getenv("ASERTO_POLICY_ROOT"),
		tenantID:             os.Getenv("ASERTO_TENANT_ID"),
	}
}

func NewAuthorizerClient(ctx context.Context, opts *options) (authz.AuthorizerClient, error) {
	client, err := grpc.New(
		ctx,
		client.WithAddr(opts.authorizerAddr),
		client.WithTenantID(opts.tenantID),
		client.WithAPIKeyAuth(opts.authorizerKey),
		client.WithCACertPath(opts.authorizerCACertPath),
	)

	if err != nil {
		return nil, err
	}

	return client, nil
}

func NewDirectoryReader(ctx context.Context, opts *options) (reader.ReaderClient, error) {
	conn, err := client.NewConnection(
		ctx,
		client.WithAddr(opts.directoryAddr),
		client.WithTenantID(opts.tenantID),
		client.WithAPIKeyAuth(opts.directoryKey),
		client.WithCACertPath(opts.directoryCACertPath),
	)

	if err != nil {
		return nil, err
	}

	return reader.NewReaderClient(conn.Conn), nil
}

func AsertoAuthorizer(authClient authz.AuthorizerClient, policy *middleware.Policy, policyRoot string) *std.Middleware {
	mw := std.New(authClient, *policy).WithPolicyFromURL(policyRoot)
	mw.Identity.JWT().FromHeader("Authorization")
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

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "subject", token.Subject())))
		})
	}
}
