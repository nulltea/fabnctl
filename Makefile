include .env
export

k3s:
	k3sup install --ip=${NODE_IP} --user=${NODE_USERNAME}
	kubectl config use-context rpi-${CLUSTER_NAME}-k3s

storage:
	kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

traefik:
	helm upgrade --install traefik -n=kube-system -f=charts/values-traefik.yaml \
		--set=ports.websecure.tls.domains.main=$DOMAIN,ports.websecure.tls.domains.sans=*.$DOMAIN \
		traefik/traefik
	helm upgrade --install -n=kube-system --set=ingress.routes.host=proxy.$DOMAIN \
		traefik-dashboard charts/ingress

hyperledger-init:
	kubectl create namespace network || echo "Namespace 'network' already exists"
	kubectl config set-context --current --namespace=network
	TARGET_ARCH=ARM64 ./network.sh init

hyperledger-deploy:
	TARGET_ARCH=ARM64 ./network.sh deploy orderer
	TARGET_ARCH=ARM64 ./network.sh deploy peer -o chipa-inu
	TARGET_ARCH=ARM64 ./network.sh deploy peer -o blueberry-go
	TARGET_ARCH=ARM64 ./network.sh deploy peer -o moon-lan
	./network.sh deploy channel -C supply-channel -p peer0 -o chipa-inu
	./network.sh deploy channel -C supply-channel -p peer0 -o blueberry-go
	./network.sh deploy channel -C supply-channel -p peer0 -o moon-lan

chaincode-deploy:
	kubectl create namespace contracts || echo "Namespace 'contracts' already exists"
	kubectl config set-context --current --namespace=contracts
	TARGET_ARCH=ARM64 ./network.sh deploy cc -o chipa-inu -p peer0 -C supply-channel --cc_name assets
	TARGET_ARCH=ARM64 ./network.sh deploy cc -o chipa-inu -p peer0 -C supply-channel --cc_name devices
	TARGET_ARCH=ARM64 ./network.sh deploy cc -o chipa-inu -p peer0 -C supply-channel --cc_name requirements
	TARGET_ARCH=ARM64 ./network.sh deploy cc -o chipa-inu -p peer0 -C supply-channel --cc_name readings