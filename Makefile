default: all

version=$(shell ver=$$(git log -n 1 --pretty=oneline --format=%D | awk -F, '{print $$1}' | awk '{print $$3}'); \
	if [ "$$ver" = "master" ] ; then \
	ver="master($$(git log -n 1 --pretty=oneline --format=%h))" ; \
	fi ; \
	echo $$ver)

client: 
	mkdir -p build
	go build -ldflags "-X main.version=${version}" ./cmd/ck-client 
	mv ck-client* ./build

server: 
	mkdir -p build
	go build -ldflags "-X main.version=${version}" ./cmd/ck-server
	mv ck-server* ./build

install:
	mv build/ck-* /usr/local/bin

wrapper-client:
	mkdir -p build
	cd cmd/ck-client-wrapper && CGO_ENABLED=1 go build -buildmode=c-shared -o ../../build/libcloak_client.so .

wrapper-server:
	mkdir -p build
	cd cmd/ck-server-wrapper && CGO_ENABLED=1 go build -buildmode=c-shared -o ../../build/libcloak_server.so .

wrappers: wrapper-client wrapper-server

all: client server

clean:
	rm -rf ./build/ck-*
	rm -rf ./build/libcloak_*.so ./build/libcloak_*.h
