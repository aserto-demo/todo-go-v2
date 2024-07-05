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

	"github.com/aserto-dev/go-aserto/client"
	"github.com/aserto-dev/go-aserto/middleware"
	"github.com/aserto-dev/go-aserto/middleware/http/std"
	authz "github.com/aserto-dev/go-authorizer/aserto/authorizer/v2"
	dsr "github.com/aserto-dev/go-directory/aserto/directory/reader/v3"

	"github.com/avast/retry-go"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"google.golang.org/grpc"
)

func main() {
	options := loadOptions()
	ctx := context.Background()

	// Initialize the Todo Store
	db, dbError := store.NewStore()
	if dbError != nil {
		log.Fatal("Failed to create store:", dbError)
	}

	var dir *directory.Directory
	var err error
	err = retry.Do(func() error {
		// Create a directory reader client
		conn, err := newConnection(ctx, &options.directory)
		if err != nil {
			log.Println("Retry: Failed to create directory connection:", err)
			return err
		}
		dir = directory.NewDirectory(conn)
		return nil
	})
	if err != nil {
		log.Fatal("Failed to create directory reader client:", err)
	}

	// Initialize the Server
	srv := server.Server{Store: db, Directory: dir}

	var authorizerClient authz.AuthorizerClient
	err = retry.Do(func() error {
		// Create an authorizer client
		authorizerClient, err = NewAuthorizerClient(ctx, &options.authorizer)
		if err != nil {
			log.Println("Retry: Failed to create authorizer client:", err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Fatal("Failed to create authorizer client:", err)
	}

	// Create authorization middleware
	mw := AsertoAuthorizer(authorizerClient,
		&middleware.Policy{
			Name:          options.policyInstanceName,
			Decision:      "allowed",
			InstanceLabel: options.policyInstanceLabel,
			Root:          options.policyRoot,
		},
		options.policyRoot,
	).WithResourceMapper(srv.TodoOwnerResourceMapper)

	router := mux.NewRouter()

	// Add JWT validation
	router.Use(JWTValidator(options.jwksKeysURL))

	// Set up routes
	router.Handle("/users/{userID}", mw.HandlerFunc(dir.GetUser)).Methods("GET")

	router.Handle("/todos", mw.HandlerFunc(srv.GetTodos)).Methods("GET")
	router.Handle("/todos/{id}", mw.HandlerFunc(srv.UpdateTodo)).Methods("PUT")
	router.Handle("/todos/{id}", mw.HandlerFunc(srv.DeleteTodo)).Methods("DELETE")

	router.Handle(
		"/todos",
		mw.Check(
			std.WithObjectType("resource-creator"),
			std.WithRelation("member"),
			std.WithObjectID("resource-creators"),
			std.WithPolicyPath("rebac.check"),
		).HandlerFunc(srv.InsertTodo)).Methods("POST")

	srv.Start(router)
}

type options struct {
	authorizer client.Config
	directory  client.Config

	policyInstanceName  string
	policyInstanceLabel string
	policyRoot          string

	jwksKeysURL string
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
		authorizer: client.Config{
			Address:    authorizerAddr,
			APIKey:     os.Getenv("ASERTO_AUTHORIZER_API_KEY"),
			CACertPath: os.ExpandEnv(getEnv("ASERTO_AUTHORIZER_GRPC_CERT_PATH", "ASERTO_GRPC_CERT_PATH")),
			TenantID:   os.Getenv("ASERTO_TENANT_ID"),
		},
		directory: client.Config{
			Address:    directoryAddr,
			APIKey:     os.Getenv("ASERTO_DIRECTORY_API_KEY"),
			CACertPath: os.ExpandEnv(getEnv("ASERTO_DIRECTORY_GRPC_CERT_PATH", "ASERTO_GRPC_CERT_PATH")),
			TenantID:   os.Getenv("ASERTO_TENANT_ID"),
		},
		jwksKeysURL:         os.Getenv("JWKS_URI"),
		policyInstanceName:  os.Getenv("ASERTO_POLICY_INSTANCE_NAME"),
		policyInstanceLabel: os.Getenv("ASERTO_POLICY_INSTANCE_LABEL"),
		policyRoot:          os.Getenv("ASERTO_POLICY_ROOT"),
	}
}

func NewAuthorizerClient(ctx context.Context, cfg *client.Config) (authz.AuthorizerClient, error) {
	conn, err := newConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return authz.NewAuthorizerClient(conn), nil
}

func NewDirectoryReader(ctx context.Context, cfg *client.Config) (dsr.ReaderClient, error) {
	conn, err := newConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return dsr.NewReaderClient(conn), nil
}

func newConnection(ctx context.Context, cfg *client.Config) (grpc.ClientConnInterface, error) {
	connectionOpts, err := cfg.ToConnectionOptions(client.NewDialOptionsProvider())
	if err != nil {
		return nil, err
	}

	conn, err := client.NewConnection(ctx, connectionOpts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func AsertoAuthorizer(authClient authz.AuthorizerClient, policy *middleware.Policy, policyRoot string) *std.Middleware {
	mw := std.New(authClient, policy).WithPolicyFromURL(policyRoot)
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
