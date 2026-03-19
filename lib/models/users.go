package models

import "github.com/mfduar8766/learnKubernetes/lib/types"

type UserModel struct {
	Id             *string                  `json:"_id,omitempty"`
	UserName       string                   `json:"userName,omitempty"`
	FirstName      string                   `json:"firstName,omitempty"`
	LastName       string                   `json:"lastName,omitempty"`
	Email          string                   `json:"email,omitempty"`
	Age            int8                     `json:"age,omitempty"`
	Password       string                   `json:"password,omitempty"`
	Roles          []types.ApplicationRoles `json:"roles,omitempty"`
	CreatedAt      string                   `json:"createdAt,omitempty"`
	UpdatedAt      string                   `json:"updatedAt,omitempty"`
	ActivationCode string                   `json:"activationCode,omitempty"`
}

type UserMessagePayloadParams struct {
	ID          string
	IdsToDelete []string
}

type Params struct {
	*UserMessagePayloadParams `json:"userMessagePayload,omitempty"`
}
