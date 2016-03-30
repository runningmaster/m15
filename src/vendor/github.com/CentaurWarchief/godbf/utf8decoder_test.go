package godbf_test

import (
	"testing"

	"github.com/CentaurWarchief/godbf"
	"github.com/stretchr/testify/assert"
)

func TestDecodeUTF8(t *testing.T) {
	dec := &godbf.UTF8Decoder{}

	out, err := dec.Decode([]byte(string("RAZÃO")))

	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, string(out), "RAZÃO")
}
