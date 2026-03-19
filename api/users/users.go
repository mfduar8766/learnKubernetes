package main

import (
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
)

type Users struct {
	log *logger.Logger
}

func NewUsers(log *logger.Logger) *Users {
	return &Users{
		log: log,
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
	userBytes, err := utils.JsonMarshall(users)
	if err != nil {
		u.log.LogErrorf("Users::GetUsers()::received marshall error: %+v", err.Error())
		return nil, err
	}
	response := models.CreateNewMessagePayload(events.UserEvents(events.GET_USERS), nil, &models.ResponsePayloadParams{
		Result: types.ResponsePayloadResults(types.SUCCESS),
		Data:   string(userBytes),
	}, nil)
	responseBytes, err := response.Marshall()
	if err != nil {
		u.log.LogErrorf("User::GetUsers()::failed to marshall: %+v", err.Error())
	}
	return responseBytes, err
}
