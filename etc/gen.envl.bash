#!/usr/bin/env bash
. env.conf

if [ -f $confloc ]; then
  confirm "Rewrite $(basename $confloc) ?" || exit 0
fi

head -n 7 env.conf | tail -n 4 > $confloc
