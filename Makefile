.PYONY: clean

clean:
	@git clean -f -d -X

build:
	go build -o backup cmd/main.go