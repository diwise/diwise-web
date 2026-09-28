package rules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

var entityKeyRe = regexp.MustCompile(`^e(\d+)_(.+)$`)
var propertyKeyRe = regexp.MustCompile(`^e(\d+)_p(\d+)_(.+)$`)

// parseRuleForm bygger en Rule från indexerade formulärfält
// (e{i}_*, e{i}_p{j}_*). Tomma entitetsblock (utan id och typ och namn)
// hoppas över; tomma properties (utan target) likaså.
func parseRuleForm(form map[string][]string) (apptransform.Rule, error) {
	var rule apptransform.Rule

	get := func(key string) string {
		if len(form[key]) == 0 {
			return ""
		}
		return strings.TrimSpace(form[key][0])
	}

	rule.Match = apptransform.Match{
		Kind:            get("kind"),
		Device:          get("device"),
		Port:            get("port"),
		SensorType:      get("sensorType"),
		Object:          get("object"),
		Resource:        get("resource"),
		Env:             get("env"),
		Type:            get("type"),
		SubType:         get("subType"),
		Tenant:          get("tenant"),
		Event:           get("event"),
		Relation:        get("relation"),
		RelationRemoved: get("relationRemoved") == "true",
		Lifecycle:       get("lifecycle"),
	}

	entityIdx := map[int]bool{}
	for key, values := range form {
		if m := entityKeyRe.FindStringSubmatch(key); m != nil {
			// Flera värden på samma fält betyder kolliderande blockindex
			// (t.ex. dubbelklick utan JS-räknare) — fail fast i stället
			// för att tyst tappa ena blockets data.
			if len(values) > 1 {
				return rule, fmt.Errorf("duplicate form fields for %q: add rows one at a time", key)
			}
			i, err := strconv.Atoi(m[1])
			if err != nil || i < 0 || i > 99 {
				return rule, fmt.Errorf("bad entity index %q", key)
			}
			entityIdx[i] = true
		}
	}

	ordered := make([]int, 0, len(entityIdx))
	for i := range entityIdx {
		ordered = append(ordered, i)
	}
	slices.Sort(ordered)

	for _, i := range ordered {
		p := func(name string) string { return get(entityFieldName(i, name)) }
		ent := apptransform.Entity{
			Name:    p("name"),
			ID:      p("id"),
			Type:    p("type"),
			Context: p("context"),
			Create:  p("create") == "true",
			Delete:  p("delete") == "true",
		}
		for _, a := range strings.Split(p("removeAttributes"), "\n") {
			if a = strings.TrimSpace(a); a != "" {
				ent.RemoveAttributes = append(ent.RemoveAttributes, a)
			}
		}

		props, err := parseProperties(form, i)
		if err != nil {
			return rule, err
		}
		ent.Properties = props

		if ent.ID == "" && ent.Type == "" && ent.Name == "" && len(ent.Properties) == 0 &&
			len(ent.RemoveAttributes) == 0 && !ent.Create && !ent.Delete {
			continue
		}
		rule.Entities = append(rule.Entities, ent)
	}

	return rule, nil
}

func entityFieldName(i int, name string) string {
	return "e" + strconv.Itoa(i) + "_" + name
}

func parseProperties(form map[string][]string, entity int) ([]apptransform.Property, error) {
	get := func(entity, index int, name string) string {
		key := "e" + strconv.Itoa(entity) + "_p" + strconv.Itoa(index) + "_" + name
		if len(form[key]) == 0 {
			return ""
		}
		return strings.TrimSpace(form[key][0])
	}

	idx := map[int]bool{}
	for key, values := range form {
		if m := propertyKeyRe.FindStringSubmatch(key); m != nil {
			if len(values) > 1 {
				return nil, fmt.Errorf("duplicate form fields for %q: add rows one at a time", key)
			}
			e, err1 := strconv.Atoi(m[1])
			j, err2 := strconv.Atoi(m[2])
			if err1 != nil || err2 != nil || e != entity || j < 0 || j > 99 {
				continue
			}
			idx[j] = true
		}
	}

	props := []apptransform.Property{}
	for j := 0; j < 100; j++ {
		if !idx[j] {
			continue
		}
		p, skip, err := parseProperty(
			get(entity, j, "target"),
			get(entity, j, "type"),
			get(entity, j, "sourceKind"),
			get(entity, j, "sourceValue"),
			get(entity, j, "unit"),
			get(entity, j, "observedAt"),
			get(entity, j, "observedBy"),
			get(entity, j, "whenPresent") == "true",
			get(entity, j, "required") == "true",
			get(entity, j, "prefix"),
			get(entity, j, "transformOp"),
			get(entity, j, "transformValue"),
			get(entity, j, "transform"),
			get(entity, j, "derive"),
			get(entity, j, "subproperties"),
		)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		props = append(props, p)
	}

	return props, nil
}

func parseProperty(target, typ, sourceKind, sourceValue, unit, observedAt, observedBy string, whenPresent, required bool, prefix, transformOp, transformValue, transformJSON, deriveJSON, subpropertiesJSON string) (apptransform.Property, bool, error) {
	var p apptransform.Property
	if target == "" {
		// Tomt block (t.ex. oanvänd blank-rad) hoppas över — om inget
		// annat satts. Satt typ/källa utan target är formulärfel.
		if typ == "" && sourceKind == "" && sourceValue == "" && transformOp == "" {
			return p, true, nil
		}
		return p, false, fmt.Errorf("property target is required")
	}

	p.Target = target
	p.Type = typ
	p.Unit = unit
	p.ObservedAt = observedAt
	p.ObservedBy = observedBy
	if whenPresent {
		p.When = "present"
	}
	p.Required = required
	p.Prefix = prefix

	switch sourceKind {
	case "resource":
		p.Resource = sourceValue
	case "object":
		p.Object = sourceValue
	case "env":
		p.Env = sourceValue
	case "name":
		p.Name = sourceValue
	case "field":
		p.Field = sourceValue
	case "ref":
		p.Ref = sourceValue
	case "value":
		p.Value = sourceValue
	case "latlon":
		p.LatLon = true
	case "timestamp":
		p.Timestamp = true
	case "const":
		v, err := parseConst(sourceValue)
		if err != nil {
			return p, false, err
		}
		p.Const = v
	default:
		return p, false, fmt.Errorf("property %q: unknown source", target)
	}

	if transformOp != "" {
		t := apptransform.Transform{Op: transformOp}
		if tv := strings.TrimSpace(transformValue); tv != "" {
			f, err := strconv.ParseFloat(tv, 64)
			if err != nil {
				return p, false, fmt.Errorf("property %q: bad transform value", target)
			}
			t.Value = &f
		}
		p.Transform = []apptransform.Transform{t}
	} else if transformJSON != "" {
		var steps []apptransform.Transform
		if err := json.Unmarshal([]byte(transformJSON), &steps); err != nil {
			return p, false, fmt.Errorf("property %q: bad transform data", target)
		}
		p.Transform = steps
	}

	if deriveJSON != "" {
		var d apptransform.Derive
		if err := json.Unmarshal([]byte(deriveJSON), &d); err != nil {
			return p, false, fmt.Errorf("property %q: bad derive data", target)
		}
		p.Derive = &d
	}
	if subpropertiesJSON != "" {
		var subs []apptransform.Property
		if err := json.Unmarshal([]byte(subpropertiesJSON), &subs); err != nil {
			return p, false, fmt.Errorf("property %q: bad subproperties data", target)
		}
		p.Subproperties = subs
	}

	return p, false, nil
}

// parseConst tolkar const-värdet: giltig JSON används som den är (mappar,
// listor, tal), annars rå sträng.
func parseConst(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("const value is required")
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v, nil
	}
	return s, nil
}

func NewRuleNewPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model := featurerules.RuleFormViewModel{
			IsNew:               true,
			Tenants:             ruleTokenTenants(r),
			Rule:                blankRule(),
			TransformConfigured: app.Transforms().Configured(),
		}
		if copyID := strings.TrimSpace(r.URL.Query().Get("copy")); copyID != "" {
			if m, err := app.Transforms().GetModel(ctx, "", copyID); err == nil {
				model.Rule = m.Rule
				model.CopyFrom = copyID
			}
		}

		content := featurerules.RuleFormPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func blankRule() apptransform.Rule {
	return apptransform.Rule{
		Match: apptransform.Match{Kind: "thing", Event: "things.v1.values"},
		Entities: []apptransform.Entity{{
			ID:   "urn:ngsi-ld:X:{{nameOrID}}",
			Type: "X",
			Properties: []apptransform.Property{
				{Target: "name", Type: "Text", Source: apptransform.Source{Field: "name"}},
			},
		}},
	}
}

func NewRuleCreatePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		form := map[string][]string(r.Form)

		// Fel vid vanlig browser-POST renderar full sida (med layout),
		// vid HTMX bara innehållet — samma mönster som lyckad save
		// (StartPage kräver inloggad ctx för innehåll).
		renderErr := func(rule apptransform.Rule, msg string) {
			content := featurerules.RuleFormPage(localizer, featurerules.RuleFormViewModel{
				IsNew:               true,
				Tenants:             ruleTokenTenants(r),
				Rule:                rule,
				ErrorMessage:        msg,
				TransformConfigured: app.Transforms().Configured(),
			})
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, http.StatusOK)
		}

		rule, err := parseRuleForm(form)
		if err != nil {
			renderErr(blankRule(), err.Error())
			return
		}

		model, err := app.Transforms().CreateModel(ctx, "", rule)
		if err != nil {
			var verr *apptransform.ValidationError
			if errors.As(err, &verr) {
				renderErr(rule, verr.Message)
				return
			}
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				writeServiceError(w, err, "")
				return
			}
			renderErr(rule, err.Error())
			return
		}

		http.Redirect(w, r, "/rules/"+model.ID+"?notice=created", http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func NewRuleSavePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		form := map[string][]string(r.Form)

		revision, err := parseRevision(r.Form.Get("revision"))
		if err != nil {
			http.Error(w, "revision is required", http.StatusBadRequest)
			return
		}

		renderErr := func(rule apptransform.Rule, msg string) {
			content := featurerules.RuleFormPage(localizer, featurerules.RuleFormViewModel{
				ID:                  id,
				Revision:            revision,
				Tenants:             ruleTokenTenants(r),
				Rule:                rule,
				ShowDelete:          true,
				ErrorMessage:        msg,
				TransformConfigured: app.Transforms().Configured(),
			})
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, http.StatusOK)
		}

		rule, err := parseRuleForm(form)
		if err != nil {
			renderErr(blankRule(), err.Error())
			return
		}

		model, err := app.Transforms().UpdateModel(ctx, "", id, rule, revision)
		if err != nil {
			var verr *apptransform.ValidationError
			if errors.As(err, &verr) {
				renderErr(rule, verr.Message)
				return
			}
			if errors.Is(err, client.ErrConflict) {
				renderErr(rule, localizer.Get("rules_conflict"))
				return
			}
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				writeServiceError(w, err, "")
				return
			}
			renderErr(rule, err.Error())
			return
		}

		http.Redirect(w, r, "/rules/"+model.ID+"?notice=updated", http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

// NewEntityBlankFragment renderar ett tomt entitetsblock (HTMX, RequireHX).
func NewEntityBlankFragment(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, _ rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		index := 0
		if n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("index"))); err == nil && n >= 0 {
			index = n
		}
		helpers.WriteComponentResponse(r.Context(), w, r, featurerules.EntityBlock(localizer, index, apptransform.Entity{}), 16*1024, http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

// NewPropertyBlankFragment renderar ett tomt property-block (HTMX, RequireHX).
func NewPropertyBlankFragment(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, _ rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		entity, index := 0, 0
		if n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("entity"))); err == nil && n >= 0 {
			entity = n
		}
		if n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("index"))); err == nil && n >= 0 {
			index = n
		}
		helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PropertyBlock(localizer, entity, index, apptransform.Property{Type: "Text"}), 16*1024, http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
