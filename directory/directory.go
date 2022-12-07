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
		} else {
			log.Printf("Failed to get user: %s", err)
			http.Error(w, "failed to get user", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Add("Content-Type", "application/json")
	encodeJSONError := json.NewEncoder(w).Encode(userAsMap(userObj))
	if encodeJSONError != nil {
		http.Error(w, encodeJSONError.Error(), http.StatusBadRequest)
		return
	}
}

func (d *Directory) UserFromIdentity(ctx context.Context, identity string) (*common.Object, error) {
	identityObj, err := d.getObject(ctx, &common.ObjectIdentifier{Key: &identity, Type: &IdentityObjectType})
	if err != nil {
		return nil, &DirectoryError{Err: err, Message: "failed to get identity", StatusCode: http.StatusInternalServerError}
	}

	relation, err := d.getRelation(
		ctx,
		&common.RelationIdentifier{
			Subject:  &common.ObjectIdentifier{Type: &UserObjectType},
			Relation: &common.RelationTypeIdentifier{Name: &IdentifierRelationType, ObjectType: &IdentityObjectType},
			Object:   &common.ObjectIdentifier{Id: &identityObj.Id},
		},
	)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, &DirectoryError{Err: err, Message: fmt.Sprintf("no user with identity [%s]", identity), StatusCode: http.StatusNotFound}
	case err != nil:
		return nil, &DirectoryError{Err: err, Message: "failed to get identity relations", StatusCode: http.StatusInternalServerError}
	}

	userObj, err := d.getObject(ctx, &common.ObjectIdentifier{Id: relation.Subject.Id})
	if err != nil {
		return nil, &DirectoryError{Err: err, Message: "failed to get user", StatusCode: http.StatusInternalServerError}
	}

	return userObj, nil
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
	userMap["id"] = user.Id
	userMap["name"] = user.DisplayName
	return userMap
}
