# diwise-web

Main repository for the diwise web application.

## Development dependencies

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/air-verse/air@latest
go install github.com/go-delve/delve/cmd/dlv@latest
```

### tailwind css

https://github.com/tailwindlabs/tailwindcss

UI-källorna finns i `internal/presentation/web/components/**/*.templ` och
`internal/presentation/web/css/input.css` (Tailwind CSS v4). Använd de gemensamma
komponenterna i `components/shared` för kort, sidrubriker, filter och tabeller.
Katalog- och regelformulär med native HTML-kontroller använder
`diwise-filter-form`/`diwise-native-form`; deras tabeller använder
`diwise-data-table`. Färger hämtas från de semantiska temavariablerna så att
kontrast och ytor fungerar i både ljust och mörkt läge.

Generera och verifiera efter ändringar:

```bash
templ generate
NODE_PATH=$(npm root -g) tailwindcss -i ./internal/presentation/web/css/input.css -o ./assets/css/diwise.css
go test ./...
go vet ./...
```

Genererad Go-kod och `assets/css/diwise.css` är git-ignorerade och ska inte
redigeras för hand. Kontrollera även smala mobilvyer och båda teman vid UI-ändringar.

### Visual Studio Code add-on

https://marketplace.visualstudio.com/items?itemName=a-h.templ

### Configuration

```bash
export DIWISEWEB_ASSET_PATH=~/<your path to>/diwise-web/assets
export OAUTH2_REALM_URL="https://<iam host>/realms/<realm name>"
export OAUTH2_CLIENT_ID="<client id>"
export OAUTH2_CLIENT_SECRET="<client secret>"
```

### Authorization and navigation

Access-object authorization is the default. The runtime OPA policy must return
`{"access":{"tenant-a":["sensors.read","things.read","transforms.read"]}}`.
The policy determines the granted scopes; the web application does not infer
permissions from a tenant list or expand wildcard strings such as `sensors.*`.

Desktop and mobile navigation require the exact scope of the destination:
`sensors.read` for sensors, `things.read` for things and the catalog, and
`transforms.read` for rules. Home and logout remain available. A scope granted
in any authorized tenant makes the navigation item visible. The complete
policy access object is kept server-side in request context for `auth.HasScope`
and `auth.HasScopeInTenant`; endpoint tenant filtering remains separate.
Menu visibility does not replace endpoint authorization.

Things-v2 routes containing a thing ID require a canonical UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`).
Malformed path IDs return HTTP 400 locally, before any request to iot-things-v2.
Map markers use a fallback icon for unrecognized container subtypes, including
template IDs, so that a missing icon cannot produce an `undefined` image URL.

Legacy tenant-list policies require `AUTHZ_ACCESS_OBJECT_ENABLED=false` or
`-authz-access-object=false`. In legacy mode, scope-gated navigation still
requires an `access` object from the policy: a `tenants` list alone cannot
prove concrete scopes and does not enable those navigation items.

### Templates and rules (PLAN002)

- Mallar: `/catalog/templates` (lista), `/catalog/templates/{id}/{version}`
  (detalj), `/catalog/templates/new` (ny version = kopia + bump; versioner är
  oföränderliga), dito `/catalog/variants...`. Läsning kräver `things.read`,
  publicering `things.create`.
- Regler: `/rules` (lista), `/rules/new`, `/rules/{id}` (redigera),
  `POST /rules/{id}/delete` (tvåsteg med seed-varning). Scopes
  `transforms.read/create/update/delete` (+ `transforms.write` för
  validate/preview-fragmenten). Tom `TRANSFORM_URL` = regelsidorna visar
  "ej konfigurerad" i stället för att anropa.
- Nya env: `TRANSFORM_URL` (iot-transform-fiware `/api/v0`, frivillig),
  befintliga `THINGS_V2_URL`, `DEV_MGMT_URL`, `THINGS_URL`, `MEASUREMENTS_URL`.
- Runtime-policyn måste tilldela motsvarande scopes i sitt access-objekt.
  Devmode-policyn tilldelar samtliga scopes för de skyddade webbrutterna,
  inklusive `sensors.create` och `transforms.read/create/update/delete/write`.

### Debug

Add to configurations in launch.json

```json
{
    "name": "Debug Diwise Web",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}/cmd/diwise-web/main.go",
    "env": {
        "DIWISEWEB_ASSET_PATH": "${workspaceFolder}/assets",
        "SERVICE_PORT": "8081",
        "OAUTH2_REALM_URL": "https://<iam host>/realms/<realm name>",
        "OAUTH2_CLIENT_ID": "<client id>",
        "OAUTH2_CLIENT_SECRET": "<client secret>"
    },
    "args": []
}
```

## Development workflow

```bash
cd diwise-web
code .
air
# open http://localhost:8080 in a browser
# go templates, css output and updated webapp binary will be generated automatically on save
```
