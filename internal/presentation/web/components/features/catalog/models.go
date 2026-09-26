package catalog

import (
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
)

// TemplateRowViewModel är en mallrad i listan (publicerad version).
type TemplateRowViewModel struct {
	ID          string
	Version     string
	Category    string
	DisplayName string
	Description string
}

// TemplatesPageViewModel är listmodellen för /catalog/templates.
type TemplatesPageViewModel struct {
	Tenants         []string
	Tenant          string
	Category        string
	CategoryOptions []featuresthings.TypeOption
	Templates       []TemplateRowViewModel
	ErrorMessage    string
}

// TemplateDetailsViewModel är detaljsidan för
// /catalog/templates/{id}/{version}.
type TemplateDetailsViewModel struct {
	Tenant       string
	Spec         appthingsv2.TemplateSpec
	ErrorMessage string
}
