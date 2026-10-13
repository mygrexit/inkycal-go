BIN     := inkycal
LDFLAGS := -s -w
PI      ?= pi@inkycal.local
ARCH    ?= arm64

.PHONY: test check build-arm64 build-armv7 build-armv6 release deploy

test:
	go vet ./...
	go test ./...

# test + static analysis + known vulnerabilities (pinned tool versions, no install needed).
check: test
	go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
	node --test internal/builder/ui/

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-arm64 ./cmd/inkycal

build-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-armv7 ./cmd/inkycal

build-armv6:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-armv6 ./cmd/inkycal

# Release assets as deploy/install.sh downloads them.
release: build-arm64 build-armv7 build-armv6
	cd dist && for a in arm64 armv7 armv6; do cp $(BIN)-$$a $(BIN)-linux-$$a; done && sha256sum $(BIN)-linux-* > SHA256SUMS

# make deploy PI=pi@192.168.1.50 ARCH=armv7
deploy: build-$(ARCH)
	scp dist/$(BIN)-$(ARCH) $(PI):/tmp/inkycal
	scp deploy/inkycal-go.service $(PI):/tmp/inkycal-go.service
	ssh $(PI) 'sudo install -m 0755 /tmp/inkycal /usr/local/bin/inkycal && sudo install -m 0644 /tmp/inkycal-go.service /etc/systemd/system/inkycal-go.service && sudo systemctl daemon-reload'
