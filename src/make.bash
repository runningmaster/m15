#!/usr/bin/env bash
. ../etc/env.conf

go list ./... | grep -v vendor/ | xargs -L1 go generate
go list ./... | grep -v vendor/ | xargs -L1 go fmt
go build -i -o $GOPATH/bin/$PROJNAME main
