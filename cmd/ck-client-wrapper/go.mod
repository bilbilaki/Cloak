module github.com/cbeuw/Cloak/cmd/ck-client-wrapper

go 1.24.0

replace github.com/bilbilaki/Cloak/hiddify-libclash/bridge => ../../hiddify-libclash/bridge

require (
	github.com/bilbilaki/Cloak/hiddify-libclash/bridge v0.0.0
	github.com/cbeuw/Cloak v0.0.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/juju/ratelimit v1.0.2 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/refraction-networking/utls v1.8.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
)

replace github.com/cbeuw/Cloak => ../..
