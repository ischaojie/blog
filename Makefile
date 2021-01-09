.PYONY: clean

clean:
	@git clean -f -d -X

build:
	@go build ./cmd/backup/

install:
	@go install ./cmd/backup/