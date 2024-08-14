package directory

import (
	"context"
	"fmt"
	"log"

	"todo-go/store"

	cerr "github.com/aserto-dev/errors"
	"github.com/aserto-dev/go-aserto/ds/v3"
	dsc "github.com/aserto-dev/go-directory/aserto/directory/common/v3"
	dsr "github.com/aserto-dev/go-directory/aserto/directory/reader/v3"
	dsw "github.com/aserto-dev/go-directory/aserto/directory/writer/v3"
	"github.com/aserto-dev/go-directory/pkg/derr"
	"github.com/pkg/errors"
)

var (
	IdentityObjectType = "identity"
	UserObjectType     = "user"

	IdentifierRelationType = "identifier"

	ErrNotFound = fmt.Errorf("not found")
)

type Todo = store.Todo

type Directory struct {
	*ds.Client
}

func NewDirectory(cfg *ds.Config) (*Directory, error) {
	client, err := cfg.Connect()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory client")
	}

	return &Directory{Client: client}, nil
}

func (d *Directory) GetUser(ctx context.Context, objID string) (*dsc.Object, error) {
	resp, err := d.Reader.GetObject(ctx, &dsr.GetObjectRequest{ObjectType: "user", ObjectId: objID})
	if err != nil {
		log.Printf("Failed to get user [%s]: %s", objID, err)
		return nil, err
	}

	return resp.Result, nil
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
