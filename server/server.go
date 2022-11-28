package server

import (
	"encoding/json"
	"log"
	"net/http"

	"todo-go/directory"
	"todo-go/store"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Todo = store.Todo

type Server struct {
	Store     *store.Store
	Directory *directory.Directory
}

func (s *Server) GetTodos(w http.ResponseWriter, r *http.Request) {
	var todos []Todo

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
	var todo Todo
	jsonErr := json.NewDecoder(r.Body).Decode(&todo)
	if jsonErr != nil {
		http.Error(w, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.Directory.UserFromIdentity(r.Context(), r.Context().Value("subject").(string))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	todo.ID = uuid.New().String()
	todo.OwnerID = user.Key

	if err := s.Store.InsertTodo(&todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(todo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *Server) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo
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
	if err := s.Store.DeleteTodo(mux.Vars(r)["id"]); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(200)
}

func (s *Server) TodoOwnerResourceMapper(r *http.Request, resource map[string]interface{}) {
	id, ok := mux.Vars(r)["id"]
	if !ok {
		return
	}

	if todo, err := s.Store.GetTodo(id); err == nil && todo != nil {
		resource["ownerID"] = todo.OwnerID
	}
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

func (s *Server) Start(handler http.Handler) {
	log.Println("Staring server on 0.0.0.0:3001")

	srv := http.Server{
		Handler: cors(handler),
		Addr:    "0.0.0.0:3001",
	}
	log.Fatal(srv.ListenAndServe())
}
