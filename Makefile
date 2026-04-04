HOSTNAME=registry.terraform.io
NAMESPACE=kenchan0130
NAME=cloudflareext
BINARY=terraform-provider-${NAME}
VERSION=$$(cat version)
OS_ARCH=$$(go env GOOS)_$$(go env GOARCH)
INSTALL_DIR=~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

default: build

.PHONY: build
build:
	go build -o ${BINARY}

.PHONY: install
install: build
	mkdir -p ${INSTALL_DIR}
	cp ${BINARY} ${INSTALL_DIR}

.PHONY: test
test:
	go vet ./...
	go test ./... -v -count=1 -race -shuffle=on

.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

.PHONY: generate
generate:
	go generate ./...

.PHONY: generate/docs
generate/docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name ${NAME}

.PHONY: fmt
fmt:
	go fmt ./...
	gofmt -s -w .

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -f ${BINARY}
	rm -rf dist/
