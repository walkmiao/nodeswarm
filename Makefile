.PHONY: test vet run build win linux-amd64 linux-arm64 all zip

APP := node-console

test:
	go test ./... -count=1

vet:
	go vet ./...

run:
	go run . -mode controller

build:
	go build -trimpath -o $(APP) .

win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o $(APP)-windows-amd64.exe .

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o $(APP)-linux-amd64 .

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o $(APP)-linux-arm64 .

all: linux-amd64 linux-arm64 win

zip:
	python3 scripts/package.py
