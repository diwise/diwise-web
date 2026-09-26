package catalog

import (
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	"strconv"
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

// TemplateNewViewModel är skapa-ny-version för /catalog/templates/new.
// Källa (From "id/version") kopieras med ny version (B3); relations,
// recipes, propertyDefs och paramInfo följer med read-only. Blankett utan
// källa sätter endast basfält + required/optional + params.
type TemplateNewViewModel struct {
	Tenants         []string
	Tenant          string
	From            string
	FromLocked      bool
	ID              string
	Version         string
	DisplayName     string
	Description     string
	Category        string
	Required        string
	Optional        string
	Geometries      []string
	AllowNoLocation bool
	ParamDefaults   string
	OverridableText string
	CopiedRelations int
	CopiedRecipes   int
	ErrorMessage    string
}

// VariantRowViewModel är en variantrad i listan.
type VariantRowViewModel struct {
	ID              string
	Version         string
	TemplateID      string
	TemplateVersion string
}

// VariantsPageViewModel är listmodellen för /catalog/variants.
type VariantsPageViewModel struct {
	Tenants      []string
	Tenant       string
	Variants     []VariantRowViewModel
	ErrorMessage string
}

// VariantDetailsViewModel är detaljsidan för
// /catalog/variants/{id}/{version}.
type VariantDetailsViewModel struct {
	Tenant       string
	Spec         appthingsv2.VariantSpec
	ErrorMessage string
}

// Float formaterar ett parametervärde utan överflödiga decimaler.
func (VariantDetailsViewModel) Float(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// VariantNewViewModel är skapa-ny-version för /catalog/variants/new.
type VariantNewViewModel struct {
	Tenants         []string
	Tenant          string
	From            string
	ID              string
	Version         string
	TemplateRef     string
	TemplateOptions []string
	ParamValues     string
	ErrorMessage    string
}
