package main

import (
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	protos "github.com/mfduar8766/learnKubernetes/lib/protos/generated"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
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

func (u *Users) GetUsers(responseTopic string) ([]byte, error) {
	userList := &protos.UserList{
		Users: []*protos.UserModel{
			{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "test@gmail.com",
				Age:       21,
				Password:  "testPassword21!",
				Roles: []protos.ApplicationRoles{
					protos.ApplicationRoles_ROLE_ADMIN,
					protos.ApplicationRoles_ROLE_SUPER_USER,
				},
				CreatedAt: time.Now().UTC().String(),
			},
			{
				FirstName: "Jane",
				LastName:  "Doe",
				Email:     "test@gmail.com",
				Age:       22,
				Password:  "testPassword22!",
				Roles: []protos.ApplicationRoles{
					protos.ApplicationRoles_ROLE_ADMIN,
					protos.ApplicationRoles_ROLE_SUPER_USER,
				},
				CreatedAt: time.Now().UTC().String(),
			},
		},
	}

	response := models.CreateNewMessagePayload(
		responseTopic,
		string(events.GET_USERS),
		nil,
		&models.ResponsePayloadParams[*protos.UserList]{
			Result: protos.ResponsePayloadResults_RESULTS_SUCCESS,
			Data:   userList,
		},
		nil,
	)
	responseBytes, err := response.Marshal()
	if err != nil {
		u.log.LogErrorf("User::GetUsers()::failed to marshall: %+v", err.Error())
		return nil, err
	}
	return responseBytes, nil
}
