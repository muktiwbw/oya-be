package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthcheckArgumentValidation(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"oya", "migrate", "extra"}
	err := run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage: oya migrate")
}
