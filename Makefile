.DEFAULT_GOAL: help

.PHONY: setup format build docker tidy test run

GO 				?= go
BINARY_NAME 	:= round-up
BUILD_OPTS 		:= GOOS=linux GOARCH=arm64
REGISTRY 		:= adamclark123/round-up
VERSION         := latest

help: ## Displays this help
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

setup: ## Sets up the module
	$(GO) mod init github.com/AClarkie/k8s-tui
	make format
	make tidy

format: ## Formats the code
	$(GO) fmt $$($(GO) list ./...)

build: ## Builds the app
	$(BUILD_OPTS) $(GO) build -o $(BINARY_NAME) cmd/main.go

docker: ## Builds the docker image
	docker build --no-cache -t $(REGISTRY):$(VERSION) .

docker-run: docker ## Run the docker image, exposed on port 8080
	docker run -it -p 8080:8080 $(REGISTRY):$(VERSION)

tidy: ## Tidies up dependencies
	$(GO) mod tidy

test: ## Runs the tests
	$(GO) test ./...

run: ## Runs the app
	$(GO) run cmd/main.go
