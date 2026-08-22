package main

import (
	"testing"

	"github.com/mfduar8766/learnKubernetes/lib/logger/mocks/logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport/mocks/transport"
	"github.com/stretchr/testify/assert"
)

func TestGetUsers(t *testing.T) {
	var tMock *transport.MockITransport = transport.NewMockITransport(t)
	var lMock *logger.MockILogger = logger.NewMockILogger(t)
	user := NewUsers(tMock, lMock)
	assert.NotNil(t, user)

	u, err := user.GetUsers("foo")
	assert.Nil(t, err)
	assert.NotNil(t, u)
}
