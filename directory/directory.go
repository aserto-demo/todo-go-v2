package directory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"todo-go/common"

	"todo-go/store"

	cerr "github.com/aserto-dev/errors"
	dsc "github.com/aserto-dev/go-directory/aserto/directory/common/v3"
	dsr "github.com/aserto-dev/go-directory/aserto/directory/reader/v3"
	dsw "github.com/aserto-dev/go-directory/aserto/directory/writer/v3"
	"github.com/aserto-dev/go-directory/pkg/derr"
	"github.com/pkg/errors"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

var (
	IdentityObjectType = "identity"
	UserObjectType     = "user"

	IdentifierRelationType = "identifier"

	ErrNotFound = fmt.Errorf("not found")
)

type Todo = store.Todo

type DirectoryError struct {
	Err        error
	Message    string
	StatusCode int
}

func (e *DirectoryError) Error() string {
	return e.Message
}

type Directory struct {
	Reader dsr.ReaderClient
	Writer dsw.WriterClient
}

func NewDirectory(conn grpc.ClientConnInterface) *Directory {
	return &Directory{
		Reader: dsr.NewReaderClient(conn),
		Writer: dsw.NewWriterClient(conn),
	}
}

func (d *Directory) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	callerPID, ok := r.Context().Value(common.ContextKeySubject).(string)
	if !ok {
		http.Error(w, "context does not contain a subject value", http.StatusExpectationFailed)
		return
	}

	var userObj *dsc.Object
	var err error
	if userID == callerPID {
		userObj, err = d.UserFromIdentity(r.Context(), userID)
	} else {
		userObj, err = d.getObject(r.Context(), "user", userID)
	}
	if err != nil {
		var dirErr *DirectoryError
		if errors.As(err, &dirErr) {
			log.Printf("%s. %s", dirErr.Message, dirErr.Err)
			http.Error(w, dirErr.Message, dirErr.StatusCode)
			return
		}

		log.Printf("Failed to get user: %s", err)
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

func (d *Directory) UserFromIdentity(ctx context.Context, identity string) (*dsc.Object, error) {
	relResp, err := d.Reader.GetRelation(ctx, &dsr.GetRelationRequest{
		SubjectType: "user",
		Relation:    "identifier",
		ObjectType:  "identity",
		ObjectId:    identity,
		WithObjects: true,
	})
	switch {
	case errors.Is(cerr.UnwrapAsertoError(err), derr.ErrRelationNotFound):
		log.Printf("identity not found [%s]", identity)
		return nil, ErrNotFound
	case err != nil:
		log.Printf("Failed to get relations for identity [%+v]: %s", identity, err)
		return nil, err
	}

	user, ok := relResp.Objects[fmt.Sprintf("%s:%s", "user", relResp.Result.SubjectId)]
	if !ok {
		return nil, errors.Wrap(ErrNotFound, "user not found")
	}

	return user, nil
}

func (d *Directory) AddTodo(ctx context.Context, todo *Todo) error {
	if _, err := d.Writer.SetObject(ctx, &dsw.SetObjectRequest{
		Object: &dsc.Object{
			Id:          todo.ID,
			Type:        "resource",
			DisplayName: todo.Title,
		},
	}); err != nil {
		log.Printf("Failed to create resource [%+v]: %s", todo.Title, err)
		return err
	}
	if _, err := d.Writer.SetRelation(ctx, &dsw.SetRelationRequest{
		Relation: &dsc.Relation{
			SubjectType: "user",
			SubjectId:   todo.OwnerID,
			Relation:    "owner",
			ObjectType:  "resource",
			ObjectId:    todo.ID,
		},
	}); err != nil {
		log.Printf("Failed to set owner relation [%+v]: %s", todo.Title, err)
		return err
	}

	return nil
}

func (d *Directory) DeleteTodo(ctx context.Context, id string) error {
	if _, err := d.Writer.DeleteObject(ctx, &dsw.DeleteObjectRequest{
		ObjectType:    "resource",
		ObjectId:      id,
		WithRelations: true,
	}); err != nil {
		log.Printf("Failed to delete todo object [%+v]: %s", id, err)
		return err
	}

	return nil
}

func (d *Directory) getObject(ctx context.Context, objType, objID string) (*dsc.Object, error) {
	resp, err := d.Reader.GetObject(ctx, &dsr.GetObjectRequest{ObjectType: objType, ObjectId: objID})
	if err != nil {
		log.Printf("Failed to get object[%s:%s]: %s", objType, objID, err)
		return nil, err
	}

	return resp.Result, nil
}

func userAsMap(user *dsc.Object) map[string]interface{} {
	userMap := user.Properties.AsMap()
	userMap["key"] = user.Id
	userMap["name"] = user.DisplayName
	return userMap
}
