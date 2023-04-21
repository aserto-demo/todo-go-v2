package directory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/aserto-dev/go-directory/aserto/directory/common/v2"
	"github.com/aserto-dev/go-directory/aserto/directory/reader/v2"
	"github.com/gorilla/mux"
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
	Reader reader.ReaderClient
}

func (d *Directory) GetUser(w http.ResponseWriter, r *http.Request) {
	identity := mux.Vars(r)["userID"]

	var dirErr *DirectoryError
	userObj, err := d.UserFromIdentity(r.Context(), identity)
	if err != nil {
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

func (d *Directory) UserFromIdentity(ctx context.Context, identity string) (*common.Object, error) {
	resp, err := d.Reader.GetRelation(ctx, &reader.GetRelationRequest{
		Param: &common.RelationIdentifier{
			Subject:  &common.ObjectIdentifier{Type: proto.String("user")},
			Relation: &common.RelationTypeIdentifier{Name: proto.String("identifier"), ObjectType: proto.String("identity")},
			Object:   &common.ObjectIdentifier{Type: proto.String("identity"), Key: &identity},
		},
		WithObjects: proto.Bool(true),
	})
	switch {
	case err != nil:
		log.Printf("Failed to get relations for identity [%+v]: %s", identity, err)
		return nil, err
	case len(resp.Results) == 0:
		log.Printf("No relations found for identity [%+v]", identity)
		return nil, ErrNotFound
	}

	return resp.Objects[*resp.Results[0].Subject.Id], nil
}

func (d *Directory) getObject(ctx context.Context, identifier *common.ObjectIdentifier) (*common.Object, error) {
	resp, err := d.Reader.GetObject(ctx, &reader.GetObjectRequest{Param: identifier})
	if err != nil {
		log.Printf("Failed to get object[%+v]: %s", identifier, err)
		return nil, err
	}

	return resp.Result, nil
}

func (d *Directory) getRelation(ctx context.Context, identifier *common.RelationIdentifier) (*common.Relation, error) {
	relationResp, err := d.Reader.GetRelations(ctx, &reader.GetRelationsRequest{Param: identifier})
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

func userAsMap(user *common.Object) map[string]interface{} {
	userMap := user.Properties.AsMap()
	userMap["key"] = user.Key
	userMap["name"] = user.DisplayName
	return userMap
}
