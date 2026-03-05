.PHONY: build run stop clean

APP=siloam

build:
	go build -o $(APP) .

run: build
	./$(APP)

clean:
	rm -f $(APP)