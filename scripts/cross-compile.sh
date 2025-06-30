#!/usr/bin/env bash

set -euxo pipefail

rm -rf ./bin

build_commands=('
    apk add make curl git \
    ; GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/alvu \
    ; GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/alvu \
    ; GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o bin/linux-arm/alvu \
    ; GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o bin/windows-386/alvu \
    ; GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/windows-amd64/alvu \
    ; GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/darwin-amd64/alvu \
    ; GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/darwin-arm64/alvu 
')

# run a docker container with osxcross and cross compile everything
docker run -it --rm -v $(pwd):/usr/local/src -w /usr/local/src \
	golang:1.24-alpine3.22 \
	sh -c "$build_commands"

# create archives
cd bin
for dir in $(ls -d *);
do
    cp ../readme.md $dir
    cp ../license $dir
    mkdir -p $dir/docs
    cp -r ../docs/pages/* $dir/docs
    tar cfzv "$dir".tgz $dir
    rm -rf $dir
done
cd ..
