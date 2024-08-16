package server

import (
	"os"

	"github.com/aserto-dev/go-aserto"
	"github.com/aserto-dev/go-aserto/ds/v3"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Options struct {
	Authorizer *aserto.Config
	Directory  *ds.Config

	PolicyName string
	PolicyRoot string

	OidcIssuer   string
	OidcAudience string
	OidcJwksURL  string

	LogLevel zerolog.Level
}

func LoadOptions() (*Options, error) {
	if err := loadEnv(); err != nil {
		return nil, err
	}

	authorizerAddr := getEnvOr("ASERTO_AUTHORIZER_SERVICE_URL", "localhost:8282")
	directoryAddr := getEnvOr("ASERTO_DIRECTORY_SERVICE_URL", "localhost:9292")
	asertoLogLevel := getEnvOr("ASERTO_LOG_LEVEL", "info")

	logLevel, err := zerolog.ParseLevel(asertoLogLevel)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid log level [%s] in ASERTO_LOG_LEVEL", asertoLogLevel)
	}

	options := &Options{
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
		PolicyName:   os.Getenv("ASERTO_POLICY_INSTANCE_NAME"),
		PolicyRoot:   getEnvOr("ASERTO_POLICY_ROOT", "todoApp"),
		OidcIssuer:   getEnvOr("ISSUER", "https://citadel.demo.aserto.com/dex"),
		OidcAudience: getEnvOr("AUDIENCE", "citadel-app"),
		OidcJwksURL:  getEnvOr("JWKS_URL", "https://citadel.demo.aserto.com/dex/keys"),
		LogLevel:     logLevel,
	}

	// Initialize logging.
	initLogging(options.LogLevel)

	log.Info().
		Str("authorizer", options.Authorizer.Address).
		Str("directory", options.Directory.Address).
		Msg("options loaded")

	return options, nil
}

func loadEnv() error {
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return errors.Wrap(godotenv.Load(), "failed to load .env file")
}

func initLogging(level zerolog.Level) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.DefaultContextLogger = &log.Logger
	zerolog.SetGlobalLevel(level)
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
