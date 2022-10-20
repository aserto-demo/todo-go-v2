// Package sqlitedb sets up the database, and handles all interactions with it
package store

import (
	"database/sql"
	"log"
	"os"

	"todo-go/structs"

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

type Todo = structs.Todo
type Store struct {
	DB *sql.DB
}

func (s *Store) GetTodos() ([]Todo, error) {
	var todos []Todo

	rows, err := s.DB.Query("SELECT * FROM todos")

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if err != nil {
		return nil, err
	} else {
		scanErr := scan.Rows(&todos, rows)
		if scanErr != nil {
			return nil, scanErr
		} else {
			return todos, nil
		}
	}
}

func (s *Store) InsertTodo(todo Todo) error {
	_, err := s.DB.Exec(`INSERT INTO todos (ID, OwnerID, Title, Completed) VALUES (?, ?, ?, ?)`, todo.ID, todo.OwnerID, todo.Title, todo.Completed)

	if err != nil {
		return err
	} else {
		return nil
	}
}

func (s *Store) UpdateTodo(todo Todo) error {
	_, err := s.DB.Exec(`UPDATE todos SET OwnerID=?, Title=?, Completed=? WHERE ID=?`, todo.OwnerID, todo.Title, todo.Completed, todo.ID)

	if err != nil {
		return err
	} else {
		return nil
	}
}

func (s *Store) DeleteTodo(todo Todo) error {
	_, err := s.DB.Exec(`DELETE FROM todos WHERE ID=?`, todo.ID)

	if err != nil {
		return err
	} else {
		return nil
	}
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
