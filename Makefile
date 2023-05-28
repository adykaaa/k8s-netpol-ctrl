test: # Runs the unit tests
	go test -v -cover ./...
.PHONY: test

test-cov: #Runs the tests and opens the coverage report
	go test -coverprofile cover.out ./...
	go tool cover -func cover.out
	go tool cover -html=cover.out -o coverage.html
.PHONY: test-cov

build: # Builds the Docker image
	docker build . -t k8s-netpol-ctrl
.PHONY: build-backend

generate-mocks: # generates all the interface mocks
	mockgen -package watcher -destination watcher/mocks/mock.go  github.com/adykaaa/k8s-netpol-ctrl/watcher EventHandler
	mockgen -package eventmock -destination handlers/event/mocks/mock.go  github.com/adykaaa/k8s-netpol-ctrl/handlers/event NetworkPolicyHandler,ObjectHandler,AttributeHandler
	mockgen -package app -destination app/mocks/mock.go  github.com/adykaaa/k8s-netpol-ctrl/app ResourceWatcher
.PHONY: generate-mocks

deploy: # deploys the controller into K8s
	kubectl apply -f deploy.yaml
.PHONY: deploy