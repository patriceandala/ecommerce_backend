package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
)

type Middleware interface {
	WrapHandler(next http.Handler) http.Handler
}

type apiMiddleware struct {
	name    string
	version string

	auth *basicAuthMiddleware
}

func (am *apiMiddleware) WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ver", am.version)
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/":
				fmt.Fprintf(w, "%s server version %s", am.name, am.version)
				return
			case "/version", "/status":
				http.Redirect(w, r, "/", 301)
				return
			}
		}
		am.auth.WrapHandler(next).ServeHTTP(w, r)
	})
}

type basicAuthMiddleware struct {
	username string
	password string
}

func (am *basicAuthMiddleware) WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))

			wantUsernameHash := sha256.Sum256([]byte(am.username))
			wantPasswordHash := sha256.Sum256([]byte(am.password))

			// validate the given credentials against the expected credentials
			if subtle.ConstantTimeCompare(usernameHash[:], wantUsernameHash[:]) == 1 &&
				subtle.ConstantTimeCompare(passwordHash[:], wantPasswordHash[:]) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}

		// fail the request if the authorization header is not present or
		// the supplied credentials are invalid.
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
