package utils

import (
	"errors"
	"os"
	"testing"

	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/stretchr/testify/assert"
)

func TestJsonMarshall(t *testing.T) {
	var testData map[string]string = map[string]string{
		"foo": "bar",
	}
	b, err := JsonMarshall(testData)
	assert.Nil(t, err)
	assert.NotNil(t, b)

	type InvalidData struct {
		Name string
		Func func() // Functions cannot be marshaled
	}

	payload := InvalidData{
		Name: "Test",
		Func: func() {},
	}

	b, err = JsonMarshall(payload)
	assert.Error(t, err)
	assert.Nil(t, b)

	err = JsonUnMarshall([]byte(nil), payload)
	assert.Error(t, err)

	date := GetDate()
	assert.NotNil(t, date)

	errMessage := BuildHttpError(errors.New("errrr"), "message", "foo", "bar")
	assert.NotNil(t, errMessage)

	port := GetHostPort(types.APP_GATE_WAY)
	assert.Equal(t, 3000, port)

	port = GetHostPort(types.APP_USERS_SERVICE)
	assert.Equal(t, 3001, port)

	err = os.Setenv(types.CURRENT_ENV, types.PROD_ENV)
	assert.NotEmpty(t, GetEnv(types.CURRENT_ENV))
	assert.Nil(t, err)
	err = os.Setenv(types.HOST_PORT, "3000")
	assert.Nil(t, err)
	port = GetHostPort("")
	assert.Equal(t, 3000, port)

	assert.NotEmpty(t, NewUUID())
}
