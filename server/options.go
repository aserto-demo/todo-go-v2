package server

import (
	"log"
	"os"

	"github.com/aserto-dev/go-aserto"
	"github.com/aserto-dev/go-aserto/ds/v3"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

type Options struct {
	Authorizer *aserto.Config
	Directory  *ds.Config

	PolicyName string
	PolicyRoot string

	OidcIssuer   string
	OidcAudience string
	OidcJwksURL  string
}

func LoadOptions() (*Options, error) {
	if err := godotenv.Load(); err != nil {
		return nil, errors.Wrap(err, "failed to load .env file")
	}

	authorizerAddr := getEnvOr("ASERTO_AUTHORIZER_SERVICE_URL", "localhost:8282")
	directoryAddr := getEnvOr("ASERTO_DIRECTORY_SERVICE_URL", "localhost:9292")

	log.Printf("Authorizer: %s\n", authorizerAddr)
	log.Printf("Directory:  %s\n", directoryAddr)

	return &Options{
		Authorizer: &aserto.Config{
			Address:    authorizerAddr,
			APIKey:     os.Getenv("ASERTO_AUTHORIZER_API_KEY"),
			CACertPath: os.ExpandEnv(getEnv("ASERTO_AUTHORIZER_GRPC_CA_CERT_PATH", "ASERTO_GRPC_CA_CERT_PATH")),
			TenantID:   os.Getenv("ASERTO_TENANT_ID"),
		},
		Directory: &ds.Config{
			Config: &aserto.Config{
				Address:    directoryAddr,
				APIKey:     os.Getenv("ASERTO_DIRECTORY_API_KEY"),
				CACertPath: os.ExpandEnv(getEnv("ASERTO_DIRECTORY_GRPC_CA_CERT_PATH", "ASERTO_GRPC_CA_CERT_PATH")),
				TenantID:   os.Getenv("ASERTO_TENANT_ID"),
			}},
		OidcIssuer:   getEnvOr("ISSUER", "https://citadel.demo.aserto.com/dex"),
		OidcAudience: getEnvOr("AUDIENCE", "citadel-app"),
		OidcJwksURL:  getEnvOr("JWKS_URL", "https://citadel.demo.aserto.com/dex/keys"),
		PolicyName:   os.Getenv("ASERTO_POLICY_INSTANCE_NAME"),
		PolicyRoot:   getEnvOr("ASERTO_POLICY_ROOT", "todoApp"),
	}, nil
}

func getEnvOr(v, defaultValue string) string {
	if val := os.Getenv(v); val != "" {
		return val
	}
	return defaultValue
}

func getEnv(vars ...string) string {
	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			return val
		}
	}
	return ""
}
