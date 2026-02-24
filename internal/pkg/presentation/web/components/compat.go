package components

import shared "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/shared"

type ComponentName string

var CurrentComponent ComponentName = "current-component"

func IconSVG(name string) string {
	return shared.IconSVG(name)
}
