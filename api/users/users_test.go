package main

import (
	"testing"

	"github.com/mfduar8766/learnKubernetes/lib/logger/mocks/i_logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport/mocks/i_transport"
	"github.com/stretchr/testify/assert"
)

func TestGetUsers(t *testing.T) {
	var tMock *i_transport.MockITransport = i_transport.NewMockITransport(t)
	var lMock *i_logger.MockILogger = i_logger.NewMockILogger(t)
	user := NewUsers(tMock, lMock)
	assert.NotNil(t, user)

	u, err := user.GetUsers("foo")
	assert.Nil(t, err)
	assert.NotNil(t, u)
}
