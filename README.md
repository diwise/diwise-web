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

### Visual Studio Code add-on

https://marketplace.visualstudio.com/items?itemName=a-h.templ

### Configuration

```bash
export DIWISEWEB_ASSET_PATH=~/<your path to>/diwise-web/assets
export OAUTH2_REALM_URL="https://<iam host>/realms/<realm name>"
export OAUTH2_CLIENT_ID="<client id>"
export OAUTH2_CLIENT_SECRET="<client secret>"
```

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
- Scopes i `config/authz.rego` (+ devmode-policyn): `transforms.read` i
  default, samtliga `transforms.*` i write-uppsättningen.

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

