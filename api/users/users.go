package main

import (
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	"github.com/mfduar8766/learnKubernetes/lib/types"
)

type Users struct {
	broker transport.ITransport
	log    logger.ILogger
}

func NewUsers(broker transport.ITransport, log logger.ILogger) *Users {
	return &Users{
		broker: broker,
		log:    log,
	}
}

func (u *Users) GetUsers() ([]byte, error) {
	var users []*models.UserModel = []*models.UserModel{
		{
			FirstName:      "John",
			LastName:       "Doe",
			Email:          "test@gmail.com",
			Age:            21,
			UserName:       "",
			Roles:          []types.ApplicationRoles{types.ApplicationRoles(types.ADMIN), types.ApplicationRoles(types.SUPER_USER)},
			Password:       "testPassword21!",
			CreatedAt:      time.Now().UTC().String(),
			UpdatedAt:      "",
			ActivationCode: "",
		},
		{
			FirstName:      "Jane",
			LastName:       "Doe",
			Email:          "test@gmail.com",
			Age:            22,
			UserName:       "",
			Roles:          []types.ApplicationRoles{types.ApplicationRoles(types.ADMIN), types.ApplicationRoles(types.SUPER_USER)},
			Password:       "testPassword22!",
			CreatedAt:      time.Now().UTC().String(),
			UpdatedAt:      "",
			ActivationCode: "",
		},
	}
	// topic := u.broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request)
	topic := u.broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users")
	response := models.CreateNewMessagePayload(topic, events.UserEvents(events.GET_USERS), nil, &models.ResponsePayloadParams[[]*models.UserModel]{
		Result: types.ResponsePayloadResults(types.SUCCESS),
		Data:   users,
	}, nil)
	responseBytes, err := response.Marshall()
	if err != nil {
		u.log.LogErrorf("User::GetUsers()::failed to marshall: %+v", err.Error())
	}
	return responseBytes, err
}
