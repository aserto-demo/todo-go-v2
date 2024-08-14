package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"todo-go/directory"
	"todo-go/identity"
	"todo-go/store"

	dsc "github.com/aserto-dev/go-directory/aserto/directory/common/v3"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type Server struct {
	Store     *store.Store
	Directory *directory.Directory

	srv *http.Server
}

func New(options *Options) (*Server, error) {
	// Initialize the Todo Store
	db, err := store.NewStore()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create store")
	}

	// Create a directory client
	dir, err := directory.NewDirectory(options.Directory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory connection")
	}

	srv := &http.Server{
		Addr:              "0.0.0.0:3001",
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	return &Server{Store: db, Directory: dir, srv: srv}, nil
}

func (s *Server) Start(handler http.Handler) {
	log.Println("Starting server on 0.0.0.0:3001")

	s.srv.Handler = cors(handler)

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen error: %+v", err)
	}
}

func (s *Server) Shutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}

	log.Println("Server stopped")
}

func (s *Server) Close() {
	if err := s.Directory.Close(); err != nil {
		log.Println("failed to close directory connection:", err)
	}

	if err := s.Store.Close(); err != nil {
		log.Println("failed to close data store", err)
	}
}

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]

	userObj, err := s.getUser(r.Context(), userID)
	if err != nil {
		log.Println("Failed to get user:", err)
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	encodeJSONError := json.NewEncoder(w).Encode(userAsMap(userObj))
	if encodeJSONError != nil {
		http.Error(w, encodeJSONError.Error(), http.StatusBadRequest)
		return
	}
}

func (s *Server) GetTodos(w http.ResponseWriter, r *http.Request) {
	var todos []store.Todo

	todos, err := s.Store.GetTodos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	jsonEncodeErr := json.NewEncoder(w).Encode(todos)
	if jsonEncodeErr != nil {
		http.Error(w, jsonEncodeErr.Error(), http.StatusBadRequest)
		return
	}
}

func (s *Server) InsertTodo(w http.ResponseWriter, r *http.Request) {
	var todo store.Todo
	jsonErr := json.NewDecoder(r.Body).Decode(&todo)
	if jsonErr != nil {
		http.Error(w, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	ownerIdentity := identity.ExtractSubject(r.Context())
	if ownerIdentity == "" {
		http.Error(w, "context does not contain a subject value", http.StatusExpectationFailed)
		return
	}

	owner, err := s.Directory.UserFromIdentity(r.Context(), ownerIdentity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	todo.ID = uuid.New().String()
	todo.OwnerID = owner.Id

	if err := s.Store.InsertTodo(&todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Directory.AddTodo(r.Context(), &todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *Server) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	var todo store.Todo
	if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	todo.ID = mux.Vars(r)["id"]

	if err := s.Store.UpdateTodo(&todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *Server) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.Directory.DeleteTodo(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Store.DeleteTodo(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(200)
}

func (s *Server) getUser(ctx context.Context, userID string) (*dsc.Object, error) {
	callerPID := identity.ExtractSubject(ctx)
	if callerPID == "" {
		return nil, errors.New("missing caller identity in request context")
	}

	if userID == callerPID {
		return s.Directory.UserFromIdentity(ctx, userID)
	}

	return s.Directory.GetUser(ctx, userID)
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Authorization")
			return
		}

		h.ServeHTTP(w, r)
	})
}

func userAsMap(user *dsc.Object) map[string]interface{} {
	userMap := user.Properties.AsMap()
	userMap["key"] = user.Id
	userMap["name"] = user.DisplayName
	return userMap
}
