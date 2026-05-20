package example.authz

  import rego.v1

  default allow := {"access": {}}

  default_scopes := [
    "sensors.read",
    "things.read",
    "admin",
  ]

  write_scopes := [
    "sensors.read",
    "sensors.update",
    "things.read",
    "things.create",
    "things.update",
    "things.delete",
    "admin",
  ]

  scopes_for(payload) := write_scopes if {
    payload["diwise-write"] == "true"
  } else := default_scopes

  allow := response if {
    [_, payload, _] := io.jwt.decode(input.token)

    response := {
      "access": {
        tenant: scopes_for(payload) |
        some tenant in payload.tenants
      }
    }
  }