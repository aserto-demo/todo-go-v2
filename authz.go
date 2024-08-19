package main

import (
	"net/http"
	"todo-go/identity"
	"todo-go/server"

	"github.com/aserto-dev/go-aserto"
	"github.com/aserto-dev/go-aserto/az"
	"github.com/aserto-dev/go-aserto/middleware"
	"github.com/aserto-dev/go-aserto/middleware/gorillaz"
	"github.com/gorilla/mux"
)

func NewAuthorizerClient(cfg *aserto.Config) (*az.Client, error) {
	opts, err := cfg.ToConnectionOptions(aserto.NewDialOptionsProvider())
	if err != nil {
		return nil, err
	}

	return az.New(opts...)
}

func AuthorizationMiddleware(azClient *az.Client, options *server.Options) *gorillaz.Middleware {
	policy := &middleware.Policy{
		Name:     options.PolicyName,
		Decision: "allowed",
	}
	// Create authorization middleware
	authz := gorillaz.New(azClient, policy).
		WithPolicyFromURL(options.PolicyRoot).
		WithResourceMapper(func(r *http.Request, resource map[string]interface{}) {
			resource["object_id"] = mux.Vars(r)["id"]
		})
	authz.Identity.Subject().FromContextValue(identity.SubjectKey)

	return authz
}
