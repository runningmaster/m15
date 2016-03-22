#!/usr/bin/env bash
. make.bash

cd $GOPATH/bin
./$PROJNAME "$1"
