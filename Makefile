.PHONY: format unit contract integration compose-integration e2e test validate vet race

format:
	go fmt ./...

unit:
	go test ./bookmarker/...

contract:
	go test -tags=contract ./tests/contract/...

integration:
	go test -tags=integration ./tests/integration/...

compose-integration:
	go test -tags=integration ./tests/integration/ -run TestCompose

e2e:
	npm run test:e2e

test: unit contract integration

vet:
	go vet ./...

race:
	go test -race ./...

validate: format test vet e2e