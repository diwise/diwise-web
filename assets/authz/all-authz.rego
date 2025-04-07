package example.authz

default allow := false

allow = response {					
    response := {
        "tenants": token.payload.tenants,
        "roles": ["create_sensor", "update_sensor", "delete_sensor", "create_thing", "update_thing", "delete_thing", "admin"]
    }
}
    
token := {"payload": payload} {
    [_, payload, _] := io.jwt.decode(input.token)
}