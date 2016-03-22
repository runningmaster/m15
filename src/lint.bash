#!/usr/bin/env bash
. ../etc/env.conf

rm -rf $GOPATH/pkg/
go list ./... | grep -v vendor/ | xargs -L1 go install
go list ./... | grep -v vendor/ | xargs -L1 gometalinter
