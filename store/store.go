// Package sqlitedb sets up the database, and handles all interactions with it
package store

import (
	"database/sql"
	"log"
	"os"

	"github.com/blockloop/scan"

	_ "github.com/mattn/go-sqlite3"
)

const dbPath = "./todo.db"

const createTodoTableSQL = `CREATE TABLE IF NOT EXISTS todos (
	ID TEXT PRIMARY KEY,
	Title TEXT NOT NULL,
	Completed BOOLEAN NOT NULL,
	OwnerID TEXT NOT NULL
);`

type Todo struct {
	ID        string
	OwnerID   string
	Title     string
	Completed bool
}

type Store struct {
	DB *sql.DB
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) GetTodos() ([]Todo, error) {
	return s.loadTodos("")
}

func (s *Store) InsertTodo(todo *Todo) error {
	_, err := s.DB.Exec(`INSERT INTO todos (ID, OwnerID, Title, Completed) VALUES (?, ?, ?, ?)`, todo.ID, todo.OwnerID, todo.Title, todo.Completed)

	if err != nil {
		return err
	}

	return nil
}

func (s *Store) GetTodo(id string) (*Todo, error) {
	todos, err := s.loadTodos(id)
	if err != nil {
		return nil, err
	}

	if len(todos) == 0 {
		return nil, nil
	}

	return &todos[0], nil
}

func (s *Store) UpdateTodo(todo *Todo) error {
	_, err := s.DB.Exec(`UPDATE todos SET  Title=?, Completed=? WHERE ID=?`, todo.Title, todo.Completed, todo.ID)

	if err != nil {
		return err
	}

	return nil
}

func (s *Store) DeleteTodo(id string) error {
	_, err := s.DB.Exec(`DELETE FROM todos WHERE ID=?`, id)

	if err != nil {
		return err
	}

	return nil
}

func (s *Store) loadTodos(id string) ([]Todo, error) {
	query := "SELECT ID, OwnerID, Title, Completed FROM todos"
	args := []interface{}{}

	if id != "" {
		query += " WHERE ID = ?"
		args = append(args, id)
	}

	rows, err := s.DB.Query(query, args...)
	switch {
	case err != nil:
		return nil, err
	case rows.Err() != nil:
		return nil, rows.Err()
	}

	var todos []Todo

	scanErr := scan.Rows(&todos, rows)
	if scanErr != nil {
		return nil, scanErr
	}

	return todos, nil
}

func NewStore() (*Store, error) {
	log.Println("Creating todo.db...")
	if _, fileExistsError := os.Stat(dbPath); os.IsNotExist(fileExistsError) {
		file, err := os.Create(dbPath)
		if err != nil {
			log.Fatal(err.Error())
		}
		file.Close()
		log.Println("todo.db created")
	}

	sqliteDatabase, _ := sql.Open("sqlite3", dbPath) // Open the created SQLite File

	createTodosTableErr := createTodosTable(sqliteDatabase)
	if createTodosTableErr != nil {
		return nil, createTodosTableErr
	}
	return &Store{DB: sqliteDatabase}, nil
}

func createTodosTable(db *sql.DB) error {
	log.Println("Create todos table...")

	statement, err := db.Prepare(createTodoTableSQL) // Prepare SQL Statement
	if err != nil {
		log.Fatal(err.Error())
	}
	_, execErr := statement.Exec()
	if execErr != nil {
		log.Fatal(execErr.Error())
		return execErr
	}
	log.Println("todos table created")
	return nil
}
