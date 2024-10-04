# diwise-web

Main repository for the diwise web application

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

