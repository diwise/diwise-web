package thingsv2

import (
	"testing"

	"github.com/matryer/is"
)

func TestClampPercentKeepsBarInRange(t *testing.T) {
	is := is.New(t)

	is.Equal(0, clampPercent(-8176.3))
	is.Equal(0, clampPercent(0))
	is.Equal(42, clampPercent(42.7))
	is.Equal(100, clampPercent(100))
	is.Equal(100, clampPercent(250))
}

func TestPercentToneMatchesV1WasteContainer(t *testing.T) {
	is := is.New(t)

	is.Equal("critical", percentTone(50))
	is.Equal("warning", percentTone(42))
	is.Equal("good", percentTone(30))
	is.Equal("good", percentTone(0))
}
