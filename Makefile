INSTALL_BIN=/usr/local/bin/fabnctl
INSTALL_DIR=$(HOME)/fabnctl

build:
	go build -v  -o ./bin/fabnctl .

install:
	sudo cp ./bin/fabnctl $(INSTALL_BIN)
	sudo mkdir $(INSTALL_DIR) || echo $(INSTALL_DIR) exists
	sudo cp -ur ./deploy/charts $(INSTALL_DIR)
	sudo cp -ur ./template $(INSTALL_DIR)
	sudo cp -ur ./.cli-config.yaml $(INSTALL_DIR)/.cli-config.yaml

install-dev: build install

install-local-path:
	kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

install-traefik:
	helm repo add traefik https://helm.traefik.io/traefik
	helm repo update
	helm upgrade --install traefik traefik/traefik

prepare-cluster: install-local-path install-traefik
