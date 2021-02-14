#!/bin/bash

set -e

COMMAND="$1"
DOMAIN=iotchain.network
ORDERER=orderer.iotchain.network
CHAINCODES_DIR=../contracts
IMAGE_REGISTRY=iotchainnetwork

function generateArtifacts(){
  helm upgrade --install artifacts charts/artifacts
  kubectl wait -n network --for=condition=complete job/artifacts.generate
  kubectl apply -n network -f charts/artifacts/artifacts-wait-job.yaml
  pod=$(kubectl get pods -n network | awk '{print $1}' | grep artifacts.wait)
  kubectl wait -n network --for=condition=ready "pod/$pod"
  kubectl cp -n network "$pod:crypto-config" crypto-config
  echo "crypto-config copied successfully!"
  kubectl delete -n network job artifacts.wait
}

function deployOrderer(){
  kubectl create -n network secret tls orderer.$DOMAIN-tls --key=crypto-config/ordererOrganizations/$DOMAIN/orderers/$ORDERER/tls/server.key \
    --cert=crypto-config/ordererOrganizations/$DOMAIN/orderers/$ORDERER/tls/server.crt \
    --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret generic orderer.$DOMAIN-ca --from-file=crypto-config/ordererOrganizations/$DOMAIN/orderers/$ORDERER/tls/ca.crt \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "tls secrets created successfully!"
  helm upgrade --install -n network orderer charts/orderer
  echo "orderer deployed successfully!"
}

function deployPeers() {
  org=$1
  kubectl create -n network secret tls "peer0.$org.$DOMAIN-tls" --key="crypto-config/peerOrganizations/$org.$DOMAIN/peers/peer0.$org.$DOMAIN/tls/server.key" \
    --cert="crypto-config/peerOrganizations/$org.$DOMAIN/peers/peer0.$org.$DOMAIN/tls/server.crt" \
    --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret generic "peer0.$org.$DOMAIN-ca" --from-file="crypto-config/peerOrganizations/$org.$DOMAIN/peers/peer0.$org.$DOMAIN/tls/ca.crt" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "tls secrets created successfully!"
  echo
  helm upgrade --install -n network --set=config.mspID="$org"MSP,config.domain="$org".iotchain.network "$org" charts/peer-org
  echo
  echo "$org peers deployed successfully!"
}

function deployChannels() {
  cli=$(kubectl get pods -n network | awk '{print $1}' | grep peer0.supplier-cli)
  kubectl exec -n network -it "$cli" -- sh -c /
     'peer channel create -c supply-channel -f ./channel-artifacts/supply-channel.tx -o orderer.iotchain.network:443 --tls true --cafile "$ORDERER_CA"'
  kubectl exec -n network -it "$cli" -- peer channel join -b supply-channel.block
  cli=$(kubectl get pods -n network | awk '{print $1}' | grep peer0.deliverer-cli)
  kubectl exec -n network -it "$cli" -- sh -c /
    'peer channel fetch newest supply-channel.block -c supply-channel -o orderer.iotchain.network:443 --tls=true --cafile "$ORDERER_CA"'
  kubectl exec -n network -it "$cli" -- peer channel join -b supply-channel.block
}

function enrollCA() {
  fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
  fabric-ca-client register -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --id.name=supplieradmin --id.secret=supplieradminpw --id.type=admin --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
}

function deployChaincode() {
  cc=$1
  org=$2
  peer=$3
  package="$peer.$org.$cc.tar.gz"
  cli=$(kubectl get pods -n network | awk '{print $1}' | grep "$peer.$org-cli")
  mkdir .tmp && cd .tmp
  echo "{\"path\":\"\",\"type\":\"external\",\"label\":\"$cc\"}" > metadata.json
  echo "{
    \"address\": \"$peer-$org-chaincode-$cc:7052\",
    \"dial_timeout\": \"10s\",
    \"tls_required\": false,
    \"client_auth_required\": false,
    \"client_key\": \"-----BEGIN EC PRIVATE KEY----- ... -----END EC PRIVATE KEY-----\",
    \"client_cert\": \"-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----\",
    \"root_cert\": \"-----BEGIN CERTIFICATE---- ... -----END CERTIFICATE-----\"
}" > connection.json
  tar cfz code.tar.gz connection.json
  tar cfz "$package" code.tar.gz metadata.json
  kubectl cp -n network "$package" "$cli:$package"
  cd .. && rm -rf .tmp
  kubectl exec -n network -it "$cli" -- peer lifecycle chaincode install "$package"
  id=$(kubectl exec -n network -it "$cli" -- peer lifecycle chaincode queryinstalled | sed -e "s/Package ID: //" -e "s/, Label: $cc//" | tail -n1)
  orderer_ca=$(kubectl exec -n network -it "$cli" -- sh -c 'echo $ORDERER_CA')
  peer_cert=$(kubectl exec -n network -it "$cli" -- sh -c 'echo $CORE_PEER_TLS_CERT_FILE')
  echo "$peer_cert"
  kubectl exec -n network -it "$cli" -- peer lifecycle chaincode approveformyorg -C supply-channel -n "$cc" --version 1.0 --init-required false --sequence 1 -o orderer.iotchain.network:443 --tls --cafile "$orderer_ca" --package-id "$id" && \
    peer lifecycle chaincode commit -C supply-channel --name "$cc" --version 1.0 --sequence 1 --init-required false --tls true -o orderer.iotchain.network:443 --tls --cafile "$orderer_ca" --peerAddresses "peer0.$org.iotchain.network:443" --tlsRootCertFiles "$peer_cert"
  image="$IMAGE_REGISTRY/cc.$cc"
  docker build -t "$image" "$CHAINCODES_DIR/$cc" && docker push "$image"
  helm upgrade --install --set=image.repository="$image,peer=$peer-$org,chaincode=$cc,ccid=$id" "$peer-$org-cc-$cc" charts/chaincode/
  kubectl exec -n network -it "$cli" -- peer chaincode invoke --isInit -C supply-channel -n "$cc" -o orderer.iotchain.network:443 --tls --cafile "$orderer_ca" --peerAddresses peer0.supplier.iotchain.network:443 --tlsRootCertFiles "$peer_cert" -c '{"Args":["List"]}' --waitForEvent
}

function cleanNetwork() {
  rm -rf ./crypto-config/* ./channel-artifacts/*
  helm uninstall -n network artifacts orderer supplier deliverer
  kubectl get pods -n network -w
}

function fetchCryptoConfig() {
    cli=$(kubectl get pods -n network | awk '{print $1}' | grep peer0.supplier-cli)
    kubectl cp -n network "$cli:crypto-config" crypto-config
}

function networkStatus() {
    kubectl get pods -n network -w
}

function cli(){
  cli=$(kubectl get pods -n network | awk '{print $1}' | grep peer0."$1"-cli)
  kubectl exec -n network -it "$cli" -- bash
}

function help() {
  echo "Usage: network.sh init | status | clean | cli "
}

# Network operations

case $COMMAND in
    "init")
      generateArtifacts
      ;;
    "deploy")
      # deploy entity
      shift
      ENTITY="$1"
      # parse flag args
      shift
      while [[ $# -ge 1 ]]
      do
        case "$1" in
          --org | -o)
            ORG="$2"
            shift
            ;;
          --peer | -p)
            PEER="$2"
            shift
            ;;
          --cc_name | -ccn)
            CC_NAME="$2"
            shift
            ;;
        esac
        shift
      done
      case "$ENTITY" in
        "orderer")
          deployOrderer
          ;;
        "peers")
          deployPeers "$ORG"
          ;;
        "channel")
          deployChannels "$ORG" "$PEER"
          ;;
        "cc")
          deployChaincode "$CC_NAME" "$ORG" "$PEER"
          ;;
      esac
      ;;
    "status")
        networkStatus
        ;;
    "clean")
        cleanNetwork
        ;;
    "cli")
        cli "$2"
        ;;
    "fetchCrypto")
        fetchCryptoConfig
        ;;
    *)
        help
        exit 1;
esac