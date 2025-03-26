package directory

import (
	"context"
	"fmt"

	"todo-go/store"

	"github.com/aserto-dev/go-aserto/ds/v3"
	dsc "github.com/aserto-dev/go-directory/aserto/directory/common/v3"
	dsr "github.com/aserto-dev/go-directory/aserto/directory/reader/v3"
	dsw "github.com/aserto-dev/go-directory/aserto/directory/writer/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	IdentityObjectType = "identity"
	UserObjectType     = "user"
	ResourceObjectType = "resource"

	OwnerRelation = "owner"

	IdentifierRelationType = "identifier"

	ErrNotFound = fmt.Errorf("not found")
)

type Todo = store.Todo

type Directory struct {
	*ds.Client
	isLegacy bool
}

func NewDirectory(cfg *ds.Config) (*Directory, error) {
	client, err := cfg.Connect()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory client")
	}

	isLegacy, err := isLegacy(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine if direction of 'identifier' relation")
	}

	return &Directory{
		Client:   client,
		isLegacy: isLegacy,
	}, nil
}

func (d *Directory) GetUser(ctx context.Context, objID string) (*dsc.Object, error) {
	resp, err := d.Reader.GetObject(ctx, &dsr.GetObjectRequest{ObjectType: "user", ObjectId: objID})
	if err != nil {
		log.Warn().Err(err).Msgf("failed to get user [%s]", objID)
		return nil, err
	}

	return resp.Result, nil
}

func (d *Directory) UserFromIdentity(ctx context.Context, identity string) (*dsc.Object, error) {
	user, err := d.resolveIdentity(ctx, identity)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			log.Warn().Msgf("identity not found [%s]", identity)
			return nil, ErrNotFound
		}
		log.Err(err).Msgf("failed to resolve user identity [%s]", identity)
		return nil, errors.Wrapf(err, "failed to resolve user identity [%s]", identity)
	}

	return user, nil
}

func (d *Directory) AddTodo(ctx context.Context, todo *Todo) error {
	if _, err := d.Writer.SetObject(ctx, &dsw.SetObjectRequest{
		Object: &dsc.Object{
			Id:          todo.ID,
			Type:        ResourceObjectType,
			DisplayName: todo.Title,
		},
	}); err != nil {
		log.Err(err).Msgf("failed to create resource [%s]", todo.Title)
		return err
	}
	if _, err := d.Writer.SetRelation(ctx, &dsw.SetRelationRequest{
		Relation: &dsc.Relation{
			SubjectType: UserObjectType,
			SubjectId:   todo.OwnerID,
			Relation:    OwnerRelation,
			ObjectType:  ResourceObjectType,
			ObjectId:    todo.ID,
		},
	}); err != nil {
		log.Err(err).Msgf("failed to set owner relation [%s]", todo.Title)
		return err
	}

	return nil
}

func (d *Directory) DeleteTodo(ctx context.Context, id string) error {
	if _, err := d.Writer.DeleteObject(ctx, &dsw.DeleteObjectRequest{
		ObjectType:    ResourceObjectType,
		ObjectId:      id,
		WithRelations: true,
	}); err != nil {
		log.Err(err).Msgf("failed to delete todo object [%s]", id)
		return err
	}

	return nil
}

func (d *Directory) resolveIdentity(ctx context.Context, identity string) (*dsc.Object, error) {
	if d.isLegacy {
		return d.resolveIdentityLegacy(ctx, identity)
	}

	relResp, err := d.Reader.GetRelation(ctx, &dsr.GetRelationRequest{
		SubjectType: IdentityObjectType,
		SubjectId:   identity,
		Relation:    IdentifierRelationType,
		ObjectType:  UserObjectType,
		WithObjects: true,
	})
	if err != nil {
		return nil, err
	}

	user, ok := relResp.Objects[fmt.Sprintf("%s:%s", relResp.Result.ObjectType, relResp.Result.ObjectId)]
	if !ok {
		return nil, errors.Wrapf(ErrNotFound, "user not found for identity [%s]", identity)
	}

	return user, nil
}

func (d *Directory) resolveIdentityLegacy(ctx context.Context, identity string) (*dsc.Object, error) {
	relResp, err := d.Reader.GetRelation(ctx, &dsr.GetRelationRequest{
		SubjectType: UserObjectType,
		Relation:    IdentifierRelationType,
		ObjectType:  IdentityObjectType,
		ObjectId:    identity,
		WithObjects: true,
	})
	if err != nil {
		return nil, err
	}

	user, ok := relResp.Objects[fmt.Sprintf("%s:%s", relResp.Result.SubjectType, relResp.Result.SubjectId)]
	if !ok {
		return nil, errors.Wrapf(ErrNotFound, "user not found for identity [%s]", identity)
	}

	return user, nil
}

func isLegacy(dsClient *ds.Client) (bool, error) {
	_, err := dsClient.Reader.GetRelation(context.Background(), &dsr.GetRelationRequest{
		ObjectId:    "todoDemoIdentity",
		ObjectType:  IdentityObjectType,
		Relation:    IdentifierRelationType,
		SubjectId:   "todoDemoUser",
		SubjectType: UserObjectType,
	})

	if err == nil {
		return true, nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return false, errors.Wrap(err, "failed to determine if directory is legacy")
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return false, nil
	case codes.NotFound:
		return true, nil
	default:
		return true, err
	}
}
