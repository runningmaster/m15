#!/usr/bin/env bash
. ../etc/env.conf

go generate internal/version
go build -i -o $GOPATH/bin/$PROJNAME main
go list ./... | grep -v vendor/ | xargs -L1 go fmt
