package actions

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (users) With(user *datastructure.User) (UserBuilder, error) {
	builder := UserBuilder{}
	if user == nil {
		return builder, fmt.Errorf("User passed is nil")
	}

	builder.User = *user
	return builder, nil
}

func (x users) GetByID(ctx context.Context, id primitive.ObjectID) (*UserBuilder, error) {
	b, err := x.Get(ctx, bson.M{"_id": id})
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (users) Get(ctx context.Context, q bson.M) (*UserBuilder, error) {
	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, q)
	err := res.Err()

	if err != nil {
		return nil, err
	}

	var user datastructure.User
	if err := res.Decode(&user); err != nil {
		return nil, err
	}

	builder := UserBuilder{
		user,
	}

	return &builder, nil
}
