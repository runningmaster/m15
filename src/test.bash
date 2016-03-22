#!/bin/bash
. ../etc/env.conf

go list ./... | grep -v vendor/ | xargs -L1 go test -race -bench . -benchmem -cover
