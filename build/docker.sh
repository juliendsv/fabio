#!/bin/bash -e 
#
# Docker image build script

# set path
export PATH=/usr/local/go/bin:$PATH

# set gopath
export GOPATH=~/go

# check go version
if [[ `go version` != 'go version go1.5.1 linux/amd64' ]]; then
	echo "Invalid go version. Want go 1.5.1"
	exit 1
fi

echo "Building fabio `go version`"
go clean
go build -tags netgo

v=`./fabio -v`
tag=magiconair/fabio

echo "Building docker image $tag:$v"
docker build -q -t $tag:$v .

echo "Building docker image $tag"
docker build -q -t $tag .

docker images
