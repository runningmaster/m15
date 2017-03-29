#!/usr/bin/env bash
. ../etc/env.conf

go list ./... | grep -v vendor/ | xargs -L1 go generate
go list ./... | grep -v vendor/ | xargs -L1 go fmt
go install -ldflags "-s -w" main
mv -f $GOPATH/bin/main $GOPATH/bin/$PROJNAME
