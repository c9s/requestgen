package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_rateLimiterRegex(t *testing.T) {
	value := "2+100/500ms"
	ok := rateLimiterRegex.MatchString(value)
	assert.True(t, ok)

	slice := rateLimiterRegex.FindStringSubmatch(value)
	assert.Len(t, slice, 4)

	assert.Equal(t, "2", slice[1])
	assert.Equal(t, "100", slice[2])
	assert.Equal(t, "500ms", slice[3])
}
