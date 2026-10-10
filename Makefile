BIN     := inkycal
LDFLAGS := -s -w
PI      ?= pi@inkycal.local
ARCH    ?= arm64

.PHONY: test build-arm64 build-armv7 deploy

test:
	go vet ./...
	go test ./...

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-arm64 ./cmd/inkycal

build-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-armv7 ./cmd/inkycal

# make deploy PI=pi@192.168.1.50 ARCH=armv7
deploy: build-$(ARCH)
	scp dist/$(BIN)-$(ARCH) $(PI):/tmp/inkycal
	scp deploy/inkycal-go.service $(PI):/tmp/inkycal-go.service
	ssh $(PI) 'sudo install -m 0755 /tmp/inkycal /usr/local/bin/inkycal && sudo install -m 0644 /tmp/inkycal-go.service /etc/systemd/system/inkycal-go.service && sudo systemctl daemon-reload'
