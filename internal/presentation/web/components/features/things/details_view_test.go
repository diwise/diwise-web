package things

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingPropertyLinesSkipsEmptyValuesWithoutRenderingContinue(t *testing.T) {
	is := is.New(t)

	component := thingPropertyLines(nil, []string{"", "Height 1.00 m", " "}, "missing")

	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)

	is.NoErr(err)
	is.True(strings.Contains(buf.String(), "Height 1.00 m"))
	is.True(!strings.Contains(buf.String(), "continue"))
}
