package mongoDb

import (
	"context"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ConnectToMongo(ctx context.Context, log *logger.Logger) (*mongo.Client, error) {
	mongoDSN, err := utils.CreateDbConnectionString(types.DB_MONGO, log)
	if err != nil {
		panic(err)
	}

	clientOptions := options.Client().ApplyURI(mongoDSN)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.LogInfof("Mongo::ConnectToMongo()::Connected to MongoDB on: %s", mongoDSN)
	return client, nil
}
