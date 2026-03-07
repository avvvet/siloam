.PHONY: build run run-siloam run-tahor backup clean

APP=siloam

build:
	go build -o $(APP) .

run: build
	./$(APP)

run-siloam: build
	./$(APP) -bot=siloam

run-tahor: build
	./$(APP) -bot=tahor

backup:
	cp siloam.db siloam_backup_$(shell date +%Y%m%d).db && echo "Backup created: siloam_backup_$(shell date +%Y%m%d).db"

clean:
	rm -f $(APP)