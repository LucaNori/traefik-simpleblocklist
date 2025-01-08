.PHONY: lint test vendor yaegi_test clean

default: lint test

lint:
	golangci-lint run

test:
	go test -v -race -cover ./...

vendor:
	go mod vendor

yaegi_test:
	yaegi test -v .

clean:
	rm -rf ./vendor
