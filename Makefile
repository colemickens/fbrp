all: build

install-systemd-service:
	sudo cp fbrp.service /etc/systemd/system/fbrp.service
	sudo systemctl daemon-reload;
	sudo systemctl stop fbrp.service; sudo systemctl start fbrp.service && sudo systemctl enable fbrp.service;

build:
	go build -o fbrp main.go

run: build
	./fbrp
