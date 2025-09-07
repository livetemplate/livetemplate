module counter

go 1.25

require (
	github.com/gorilla/websocket v1.5.3
	github.com/livefir/livetemplate v0.0.0
)

require github.com/golang-jwt/jwt/v5 v5.3.0 // indirect

replace github.com/livefir/livetemplate => ../..
