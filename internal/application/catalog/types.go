// Package catalog publishes thing templates and variants.
// Types are shared with thingsv2 (single source, no copies): aliases below
// keep catalog imports stable while the canonical JSON shapes live in
// internal/application/thingsv2 (which mirrors iot-things-v2 catalog +
// openapi.yaml without field loss: operatorVersions, presetProperties,
// alternativeName/subType, recipes, paramValues/paramSources).
package catalog

import (
	"github.com/diwise/diwise-web/internal/application/thingsv2"
)

// Shared contract aliases (additive fields only; do not fork shapes).
type (
	TemplateSpec    = thingsv2.TemplateSpec
	VariantSpec     = thingsv2.VariantSpec
	Template        = thingsv2.Template
	Variant         = thingsv2.Variant
	PropertyDef     = thingsv2.PropertyDef
	SignalHint      = thingsv2.SignalHint
	RelationSpec    = thingsv2.RelationSpec
	Recipe          = thingsv2.Recipe
	ParamDef        = thingsv2.ParamDef
	ObjectSpec      = thingsv2.ObjectSpec
	EffectiveConfig = thingsv2.EffectiveConfig
)
