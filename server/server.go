package server

import (
	"encoding/json"
	"log"
	"net/http"

	"todo-go/store"
	"todo-go/structs"
)

type Todo = structs.Todo

type Server struct {
	Store *store.Store
}

func (s *Server) GetTodos(w http.ResponseWriter, r *http.Request) {
	var todos []Todo

	todos, err := s.Store.GetTodos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.Header().Add("Content-Type", "application/json")
		jsonEncodeErr := json.NewEncoder(w).Encode(todos)
		if jsonEncodeErr != nil {
			http.Error(w, jsonEncodeErr.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (s *Server) InsertTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo
	jsonErr := json.NewDecoder(r.Body).Decode(&todo)
	if jsonErr != nil {
		http.Error(w, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	err := s.Store.InsertTodo(todo)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		jsonEncodeErr := json.NewEncoder(w).Encode(todo)
		if jsonEncodeErr != nil {
			http.Error(w, jsonEncodeErr.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (s *Server) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo
	jsonErr := json.NewDecoder(r.Body).Decode(&todo)
	if jsonErr != nil {
		http.Error(w, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	err := s.Store.UpdateTodo(todo)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		jsonEncodeErr := json.NewEncoder(w).Encode(todo)
		if jsonEncodeErr != nil {
			http.Error(w, jsonEncodeErr.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (s *Server) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo
	jsonErr := json.NewDecoder(r.Body).Decode(&todo)
	if jsonErr != nil {
		http.Error(w, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	err := s.Store.DeleteTodo(todo)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.WriteHeader(200)
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
		} else {
			h.ServeHTTP(w, r)
		}
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
