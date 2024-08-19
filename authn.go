package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"todo-go/identity"
	"todo-go/server"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func AuthenticationMiddleware(ctx context.Context, options *server.Options) mux.MiddlewareFunc {
	cache := jwk.NewCache(ctx)
	cache.Register(options.OidcJwksURL)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keys, err := cache.Get(r.Context(), options.OidcJwksURL)
			if err != nil || keys == nil {
				log.Printf("Failed to fetch JWKs from [%s]: %+v", options.OidcJwksURL, err)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			authorizationHeader := r.Header.Get("Authorization")
			tokenStr, _ := strings.CutPrefix(authorizationHeader, "Bearer ")

			token, err := jwt.ParseString(tokenStr,
				jwt.WithKeySet(keys),
				jwt.WithAudience(options.OidcAudience),
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			ctxWithIdentity := identity.WithSubject(r.Context(), token.Subject())

			next.ServeHTTP(w, r.WithContext(ctxWithIdentity))
		})
	}
}
