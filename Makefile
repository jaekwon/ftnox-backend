run: server

server: $(shell find . -name '*.go')
	rm -f $(GOPATH)/bin/ftnox.com
	go get -tags "server" ./...
	go run -tags "server" bin/server.go

setup: $(shell find . -name '*.go')
	go get -tags "setup" ./...
	go run -tags "setup" bin/setup.go

scrap: $(shell find . -name '*.go')
	go get -tags "scrap" ./...
	go run -tags "scrap" bin/scrap.go

scss: .FORCE
	scss scss/style.scss:static/main/css/style.css

test:
	go test ./...

lint:
	go get github.com/golang/lint/golint
	$(GOPATH)/bin/golint *.go

static: .FORCE
	handlebars templates/main/*.html -f js/templates/main.js
	handlebars templates/treasury/*.html -f js/templates/treasury.js
	node js/bin/package_main.js > static/main/js/main.js
	node js/bin/package_treasury.js > static/treasury/js/main.js
	node js/bin/render.js

.FORCE:
