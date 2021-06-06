include .env
export

k3s:
	k3sup install --ip=${NODE_IP} --user=${NODE_USERNAME}
	kubectl config use-context rpi-${CLUSTER_NAME}-k3s

storage:
	kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

traefik:
	helm upgrade --install -n=kube-system traefik-ingress charts/ingress

hyperledger-init:
	#kubectl create namespace network
	kubectl config set-context --current --namespace=network
	TARGET_ARCH=ARM64 ./network.sh init

hyperledger-deploy:
	TARGET_ARCH=ARM64 ./network.sh deploy orderer
	TARGET_ARCH=ARM64 ./network.sh deploy peer -o=supplier
	./network.sh deploy channel -C=supply-channel -p=peer0 -o=supplier
