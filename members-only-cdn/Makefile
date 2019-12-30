.PHONY: deps clean build

deps:
	go get -u ./...

clean: 
	rm -rf ./members-only-cdn
	
build:
	GOOS=linux GOARCH=amd64 go build -o members-only-cdn ./
