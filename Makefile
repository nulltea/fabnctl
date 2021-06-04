include .env
export

make k3s:
	k3sup install --ip=${NODE_IP} --user=${NODE_USERNAME}
	kubectl config use-context rpi-${CLUSTER_NAME}-k3s

make storage:
	kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

make traefik:
	helm upgrade --install -n=kube-system traefik-ingress charts/ingress