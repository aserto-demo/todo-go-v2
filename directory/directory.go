package directory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"todo-go/common"

	dsc "github.com/aserto-dev/go-directory/aserto/directory/common/v2"
	dsr "github.com/aserto-dev/go-directory/aserto/directory/reader/v2"
	dsw "github.com/aserto-dev/go-directory/aserto/directory/writer/v2"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

var (
	IdentityObjectType = "identity"
	UserObjectType     = "user"

	IdentifierRelationType = "identifier"

	ErrNotFound = fmt.Errorf("not found")
)

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
	userKey := mux.Vars(r)["userID"]
	callerPID, ok := r.Context().Value(common.ContextKeySubject).(string)
	if !ok {
		http.Error(w, "context does not contain a subject value", http.StatusExpectationFailed)
		return
	}

	var userObj *dsc.Object
	var err error
	if userKey == callerPID {
		userObj, err = d.UserFromIdentity(r.Context(), userKey)
	} else {
		userObj, err = d.getObject(r.Context(), &dsc.ObjectIdentifier{Type: proto.String("user"), Key: &userKey})
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
		Param: &dsc.RelationIdentifier{
			Subject:  &dsc.ObjectIdentifier{Type: proto.String("user")},
			Relation: &dsc.RelationTypeIdentifier{Name: proto.String("identifier"), ObjectType: proto.String("identity")},
			Object:   &dsc.ObjectIdentifier{Type: proto.String("identity"), Key: &identity},
		},
	})
	switch {
	case err != nil:
		log.Printf("Failed to get relations for identity [%+v]: %s", identity, err)
		return nil, err
	case len(relResp.Results) == 0:
		log.Printf("No relations found for identity [%+v]", identity)
		return nil, ErrNotFound
	}

	objResp, err := d.Reader.GetObject(ctx, &dsr.GetObjectRequest{Param: relResp.Results[0].Subject})
	if err != nil {
		log.Printf("Failed to get user object [%+v]: %s", relResp.Results[0].Subject, err)
		return nil, err
	}

	return objResp.Result, nil
}

func (d *Directory) AddTodo(ctx context.Context, id, owner string) error {
	if _, err := d.Writer.SetRelation(ctx, &dsw.SetRelationRequest{
		Relation: &dsc.Relation{
			Subject:  &dsc.ObjectIdentifier{Type: proto.String("user"), Key: &owner},
			Relation: "owner",
			Object:   &dsc.ObjectIdentifier{Type: proto.String("todo"), Key: &id},
		},
	}); err != nil {
		log.Printf("Failed to set owner relation [%+v]: %s", id, err)
		return err
	}

	return nil
}

func (d *Directory) DeleteTodo(ctx context.Context, id string) error {
	if _, err := d.Writer.DeleteObject(ctx, &dsw.DeleteObjectRequest{
		Param:         &dsc.ObjectIdentifier{Type: proto.String("todo"), Key: &id},
		WithRelations: proto.Bool(true),
	}); err != nil {
		log.Printf("Failed to delete todo object [%+v]: %s", id, err)
		return err
	}

	return nil
}

func (d *Directory) getObject(ctx context.Context, identifier *dsc.ObjectIdentifier) (*dsc.Object, error) {
	resp, err := d.Reader.GetObject(ctx, &dsr.GetObjectRequest{Param: identifier})
	if err != nil {
		log.Printf("Failed to get object[%+v]: %s", identifier, err)
		return nil, err
	}

	return resp.Result, nil
}

// nolint: unused
func (d *Directory) getRelation(ctx context.Context, identifier *dsc.RelationIdentifier) (*dsc.Relation, error) {
	relationResp, err := d.Reader.GetRelations(ctx, &dsr.GetRelationsRequest{Param: identifier})
	switch {
	case err != nil:
		log.Printf("Failed to get relations for [%+v]: %s", identifier, err)
		return nil, err
	case len(relationResp.Results) == 0:
		log.Printf("No relations found for [%+v]", identifier)
		return nil, ErrNotFound
	}

	return relationResp.Results[0], nil
}

func userAsMap(user *dsc.Object) map[string]interface{} {
	userMap := user.Properties.AsMap()
	userMap["key"] = user.Key
	userMap["name"] = user.DisplayName
	return userMap
}
