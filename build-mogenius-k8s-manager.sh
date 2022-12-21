#!/bin/bash

dir=$(dirname -- "$( readlink -f -- "$0"; )";)

cd ${dir}/
mkdir ${dir}/bin

echo "Build mogenius-k8s-manager"
go mod download

go build -ldflags="-extldflags= -s -w" -o ${dir}/bin/mogenius-k8s-manager .

echo "Run mogenius-k8s-manager: "
echo "${dir}/bin/mogenius-k8s-manager start"
