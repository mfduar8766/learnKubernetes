package mongoDb

import (
	"context"
	"fmt"
	"os"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ConnectToMongo(ctx context.Context, log *logger.Logger) (*mongo.Client, error) {
	defaultHost := "mongo-service:27017"

	user := os.Getenv("MONGO_INITDB_ROOT_USERNAME")
	pass := os.Getenv("MONGO_INITDB_ROOT_PASSWORD")
	host := os.Getenv("MONGO_HOST")

	if host == "" || user == "" || pass == "" {
		log.LogWarnf("Mongo::ConnectToMongo()::Mongo env vars missing, using defaultHost: %s", defaultHost)
		if host == "" {
			host = defaultHost
		}
		if user == "" {
			user = "user"
		}
		if pass == "" {
			pass = "password"
		}
	}

	mongoDSN := fmt.Sprintf("mongodb://%s:%s@%s", user, pass, host)

	log.LogInfof("Mongo::ConnectToMongo()::Attempting Mongo connection: mongodb://%s:****@%s", user, host)

	clientOptions := options.Client().ApplyURI(mongoDSN)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.LogInfof("Mongo::ConnectToMongo()::Connected to MongoDB")
	return client, nil
}
