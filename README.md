# diwise-web

Main repository for the diwise web application

## Development dependencies

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/cosmtrek/air@latest
go install github.com/go-delve/delve/cmd/dlv@latest
```

### tailwind css
https://github.com/tailwindlabs/tailwindcss

### Visual Studio Code add-on
https://marketplace.visualstudio.com/items?itemName=a-h.templ

### Configuration

```bash
export DIWISEWEB_ASSET_PATH=~/<your path to>/diwise-web/assets
export TOKEN=<a valid jwt token to a diwise instance>
```

### Debug

Add to configurations in launch.json

```json
{
    "name": "Attach to Air",
    "type": "go",
    "mode": "remote",
    "request": "attach",
    "host": "127.0.0.1",
    "port": 2345
}
```

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
        "TOKEN": "replaceme"
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
