package db

import (
	"context"
	"log"
	"time"

	"telegram-approval-bot/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Database struct {
	Client   *mongo.Client
	Users    *mongo.Collection
	Channels *mongo.Collection
}

func InitDB(uri string) *Database {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMaxPoolSize(50))
	if err != nil {
		log.Fatalf("Critical: Database connection failed: %v", err)
	}

	// Verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Critical: Database ping failed: %v", err)
	}

	db := client.Database("botdb")

	d := &Database{
		Client:   client,
		Users:    db.Collection("users"),
		Channels: db.Collection("channels"),
	}

	_, _ = d.Users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true),
	})
	return d
}

func (d *Database) SaveUserAsync(user models.User) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.Users.UpdateOne(ctx, bson.M{"user_id": user.UserID}, bson.M{"$setOnInsert": user}, options.Update().SetUpsert(true))
	}()
}

func (d *Database) AddChannelAsync(channel models.Channel) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.Channels.UpdateOne(ctx, bson.M{"channel_id": channel.ChannelID}, bson.M{"$setOnInsert": channel}, options.Update().SetUpsert(true))
	}()
}

func (d *Database) GetAllUserIDs() ([]int64, error) {
	cursor, err := d.Users.Find(context.Background(), bson.M{}, options.Find().SetProjection(bson.M{"user_id": 1}))
	if err != nil {
		return nil, err
	}
	var results []struct {
		UserID int64 `bson:"user_id"`
	}
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}
	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.UserID
	}
	return ids, nil
}
