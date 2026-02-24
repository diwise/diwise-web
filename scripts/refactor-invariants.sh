#!/usr/bin/env bash
set -euo pipefail

# Render-sensitive invariants for template refactor.
# Fails fast when known fragile patterns drift.

fail() {
  printf 'invariant-fail: %s\n' "$1" >&2
  exit 1
}

has() {
  local pattern="$1"
  local path="$2"
  rg -n --fixed-strings "$pattern" "$path" >/dev/null 2>&1
}

# 1) Map toggle icon must keep raw icon path rendering (original behavior).
if [ -f internal/pkg/presentation/web/components/shared/table_or_map.templ ]; then
  has '@templ.Raw(iconSVG("map"))' internal/pkg/presentation/web/components/shared/table_or_map.templ \
    || fail 'shared/table_or_map.templ must keep @templ.Raw(iconSVG("map")) in TableOrMap map button'
elif [ -f internal/pkg/presentation/web/components/features/sensors/sensors.templ ]; then
  has '@templ.Raw(comp.IconSVG("map"))' internal/pkg/presentation/web/components/features/sensors/sensors.templ \
    || fail 'features/sensors/sensors.templ must keep @templ.Raw(comp.IconSVG("map")) in TableOrMap map button'
elif [ -f internal/pkg/presentation/web/components/table_or_map.templ ]; then
  has '@templ.Raw(iconSVG("map"))' internal/pkg/presentation/web/components/table_or_map.templ \
    || fail 'components/table_or_map.templ must keep @templ.Raw(iconSVG("map")) in TableOrMap map button'
fi

# 2) Prevent accidental normalization of the map raw icon into shared.SVG in map button contexts.
if [ -f internal/pkg/presentation/web/components/shared/table_or_map.templ ]; then
  if rg -n --fixed-strings '@shared.SVG("map"' internal/pkg/presentation/web/components/shared/table_or_map.templ >/dev/null 2>&1; then
    fail 'TableOrMap map icon must not be converted to @shared.SVG("map", ...)'
  fi
fi

# 3) Components package must keep exported compatibility helper for feature packages.
if [ -f internal/pkg/presentation/web/components/body.templ ]; then
  has 'func IconSVG(name string) string {' internal/pkg/presentation/web/components/body.templ \
    || fail 'components/body.templ must expose IconSVG compatibility helper'
elif [ -f internal/pkg/presentation/web/components/compat.go ]; then
  has 'func IconSVG(name string) string {' internal/pkg/presentation/web/components/compat.go \
    || fail 'components/compat.go must expose IconSVG compatibility helper'
else
  fail 'components package must expose IconSVG compatibility helper'
fi

# 4) Things edit/details templates keep raw-icon datalist chevron rendering (historically fragile).
if [ -f internal/pkg/presentation/web/components/features/things/thingdetails_edit.templ ]; then
  has '@templ.Raw(comp.IconSVG("chevron-down"))' internal/pkg/presentation/web/components/features/things/thingdetails_edit.templ \
    || fail 'features/things/thingdetails_edit.templ must keep raw chevron icon rendering in datalist controls'
elif [ -f internal/pkg/presentation/web/components/thingdetails_edit.templ ]; then
  has '@templ.Raw(iconSVG("chevron-down"))' internal/pkg/presentation/web/components/thingdetails_edit.templ \
    || fail 'components/thingdetails_edit.templ must keep raw chevron icon rendering in datalist controls'
fi

# 5) Things details map still uses map-data helper path used by JS hooks.
if [ -f internal/pkg/presentation/web/components/features/things/thingdetails.templ ]; then
  if ! has '@shared.Map("small", false, false, shared.NewMapData(model.Thing.Latitude, model.Thing.Longitude), thingsToMapFeature(l10n, []ThingViewModel{model.Thing}))' internal/pkg/presentation/web/components/features/things/thingdetails.templ \
    && ! has '@comp.Map("small", false, false, comp.NewMapData(model.Thing.Latitude, model.Thing.Longitude), thingsToMapFeature(l10n, []ThingViewModel{model.Thing}))' internal/pkg/presentation/web/components/features/things/thingdetails.templ; then
    fail 'features/things/thingdetails.templ must keep small map wiring with NewMapData(...) and thingsToMapFeature(...)'
  fi
elif [ -f internal/pkg/presentation/web/components/thingdetails.templ ]; then
  has '@Map("small", false, false, newMapData(model.Thing.Latitude, model.Thing.Longitude), thingsToMapFeature(l10n, []ThingViewModel{model.Thing}))' internal/pkg/presentation/web/components/thingdetails.templ \
    || fail 'components/thingdetails.templ must keep small map wiring with newMapData(...) and thingsToMapFeature(...)'
fi

printf 'invariants-ok\n'
