#!/bin/bash

set -e

PROJECT_DIR=$PWD

ARGS_NUMBER="$#"
COMMAND="$1"

OS_ARCH=$(echo "$(uname -s|tr '[:upper:]' '[:lower:]'|sed 's/mingw64_nt.*/windows/')-$(uname -m | sed 's/x86_64/amd64/g')" | awk '{print tolower($0)}')
FABRIC_ROOT=$GOPATH/src/github.com/hyperledger/fabric

function generateArtifacts(){
  helm upgrade --install artifacts charts/artifacts
}

function deployOrderer(){
  helm upgrade --install orderer charts/orderer
}

function deployPeers(){
  helm upgrade --install --set=config.mspID=supplierMSP,config.domain=supplier,config.peerSubdomain=peer0 peer0-supplier charts/peer-org
  helm upgrade --install --set=config.mspID=delivererMSP,config.domain=deliverer,config.peerSubdomain=peer0 peer0-deliverer charts/peer-org
}

function deployChannels() {
  cli=$(kubectl get pods | awk '{print $1}' | grep peer0-supplier-cli)
  kubectl exec -it "$cli" -- peer channel create -c supply-channel -f ./channel-artifacts/supply-channel.tx \
    -o orderer:7050 --tls true --cafile $ORDERER_CA
  kubectl exec -it "$cli" -- peer channel join -b supply-channel.block
  cli=$(kubectl get pods | awk '{print $1}' | grep peer0-deliverer-cli)
  kubectl exec -it "$cli" -- peer channel fetch newest supply-channel.block -c supply-channel -o=orderer:7050 --tls=true --cafile=$ORDERER_CA
  kubectl exec -it "$cli" -- peer channel join -b supply-channel.block
}

function enrollCA() {
  fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
  fabric-ca-client register -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --id.name=supplieradmin --id.secret=supplieradminpw --id.type=admin --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
}

function deployChaincode() {
  cli=$(kubectl get pods | awk '{print $1}' | grep peer0-supplier-cli)
  package="$1.tar.gz"
  mkdir .tmp && cd .tmp
  echo "{\"path\":\"\",\"type\":\"external\",\"label\":\"$1\"}" > metadata.json
  echo "{
    \"address\": \"peer0-supplier-chaincode-$1:7052\",
    \"dial_timeout\": \"10s\",
    \"tls_required\": false,
    \"client_auth_required\": false,
    \"client_key\": \"-----BEGIN EC PRIVATE KEY----- ... -----END EC PRIVATE KEY-----\",
    \"client_cert\": \"-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----\",
    \"root_cert\": \"-----BEGIN CERTIFICATE---- ... -----END CERTIFICATE-----\"
}" > connection.json
  tar cfz code.tar.gz connection.json
  tar cfz "$package" code.tar.gz metadata.json
  kubectl cp "$package" "$cli:$package"
  cd .. && rm -rf .tmp
  # kubectl exec -it "$cli" -- peer lifecycle chaincode install chaincodes/assets.tar.gz
}

function cleanNetwork() {
  rm -rf ./crypto-config/* ./channel-artifacts/*
  helm uninstall artifacts orderer peer0-supplier peer0-deliverer
  kubectl get pods -w
}

function networkStatus() {
    kubectl get pods -w
}

function cli(){
  cli=$(kubectl get pods | awk '{print $1}' | grep peer0-"$1"-cli)
  kubectl exec -it "$cli" -- bash
}

# Network operations

case $COMMAND in
    "init")
        generateArtifacts
        ;;
    "deploy")
        case "$2" in
          "orderer")
            deployOrderer
            ;;
          "peers")
            deployPeers
            ;;
          "channel")
            deployChannels
            ;;
          "cc")
            deployChaincode "$3"
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
    *)
        echo "Usage: network.sh init | status | clean | cli "
        exit 1;
esac