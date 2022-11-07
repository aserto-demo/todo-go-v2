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

	"github.com/aserto-dev/aserto-go/authorizer/grpc"
	"github.com/aserto-dev/aserto-go/client"
	"github.com/aserto-dev/go-directory/aserto/directory/reader/v2"

	"github.com/aserto-dev/aserto-go/middleware"
	"github.com/aserto-dev/aserto-go/middleware/http/std"
	authz "github.com/aserto-dev/go-authorizer/aserto/authorizer/v2"

	"github.com/gorilla/mux"

	"todo-go/directory"
	"todo-go/server"
	"todo-go/store"
)

func AsertoAuthorizer(authClient authz.AuthorizerClient, policy *middleware.Policy, policyRoot string) *std.Middleware {
	mw := std.New(
		authClient,
		*policy,
	)

	mw.Identity.JWT().FromHeader("Authorization")
	mw.WithPolicyFromURL(policyRoot)
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
			_, err = jwt.Parse(tokenBytes, jwt.WithKeySet(keys))
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	// Load environment variables
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

	jwksKeysUrl := os.Getenv("JWKS_URI")

	policyName := os.Getenv("ASERTO_POLICY_INSTANCE_NAME")
	policyInstanceLabel := os.Getenv("ASERTO_POLICY_INSTANCE_LABEL")
	if policyInstanceLabel == "" {
		policyInstanceLabel = policyName
	}
	policyRoot := os.Getenv("ASERTO_POLICY_ROOT")
	insecure := (os.Getenv("ASERTO_INSECURE") != "")
	decision := "allowed"

	tenantID := os.Getenv("ASERTO_TENANT_ID")
	authorizerKey := os.Getenv("ASERTO_AUTHORIZER_API_KEY")
	directoryKey := os.Getenv("ASERTO_DIRECTORY_API_KEY")

	ctx := context.Background()

	// Create an authorizer client
	authorizerClient, err := NewAuthorizerClient(ctx, authorizerAddr, tenantID, authorizerKey, insecure)
	if err != nil {
		log.Fatal("Failed to create authorizer client:", err)
	}

	// Create authorization middleware
	authorizationMiddleware := AsertoAuthorizer(authorizerClient,
		&middleware.Policy{
			Name:          policyName,
			Decision:      decision,
			InstanceLabel: policyInstanceLabel,
		},
		policyRoot,
	)

	// Create a directory reader client
	directoryReader, err := NewDirectoryReader(ctx, directoryAddr, tenantID, directoryKey, insecure)
	if err != nil {
		log.Fatal("Failed to create directory reader client:", err)
	}

	dir := directory.Directory{Reader: directoryReader}

	// Initialize the Todo Store
	db, dbError := store.NewStore()
	if dbError != nil {
		log.Fatal("Failed to create store:", dbError)
	}

	// Initialize the Server
	srv := server.Server{Store: db}

	// Set up routes
	router := mux.NewRouter()
	router.HandleFunc("/todos", srv.GetTodos).Methods("GET")
	router.HandleFunc("/todo", srv.InsertTodo).Methods("POST")
	router.HandleFunc("/todo/{ownerID}", srv.UpdateTodo).Methods("PUT")
	router.HandleFunc("/todo/{ownerID}", srv.DeleteTodo).Methods("DELETE")
	router.HandleFunc("/user/{userID}", dir.GetUser).Methods("GET")

	// Add JWT validation and authorization middleware
	router.Use(JWTValidator(jwksKeysUrl), authorizationMiddleware.Handler)

	srv.Start(router)
}

func NewAuthorizerClient(ctx context.Context, addr, tenantID, apiKey string, insecure bool) (authz.AuthorizerClient, error) {
	client, err := grpc.New(
		ctx,
		client.WithAddr(addr),
		client.WithTenantID(tenantID),
		client.WithAPIKeyAuth(apiKey),
		client.WithInsecure(insecure),
	)

	if err != nil {
		return nil, err
	}

	return client, nil
}

func NewDirectoryReader(ctx context.Context, addr, tenantID, apiKey string, insecure bool) (reader.ReaderClient, error) {
	conn, err := client.NewConnection(
		ctx,
		client.WithAddr(addr),
		client.WithTenantID(tenantID),
		client.WithAPIKeyAuth(apiKey),
		client.WithInsecure(insecure),
	)

	if err != nil {
		return nil, err
	}

	return reader.NewReaderClient(conn.Conn), nil
}
