BINARY_NAME=./bin/Bot
INSTALL_DIR=/usr/local/bin/

build:
	go build -o $(BINARY_NAME) main.go

install: build
	cp $(BINARY_NAME) $(INSTALL_DIR)

clean:
	rm -f $(BINARY_NAME)

.PHONY: build install clean