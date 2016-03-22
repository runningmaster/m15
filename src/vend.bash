#!/usr/bin/env bash
. ../etc/env.conf

confirm "Update vendor packages?" || exit 0
VENDOR=$GOPATH/src/vendor

# cleanup
find $VENDOR -mindepth 1 -maxdepth 1 -type d | xargs rm -rf

# download
GOPATH=$VENDOR
while read pkg
do
	[[ $pkg =~ ^#.*$ || $pkg = "" ]] && continue
	go get -d $pkg
	echo " ->" $pkg "- OK"
done < vendor.ls

# move src
if [ -d "$VENDOR/src" ]; then
	cp -rf $VENDOR/src/* $VENDOR  && rm -rf $VENDOR/src
fi

# save git rev
find $VENDOR -type d -name ".git" -print0 | while IFS= read -r -d $'\0' line; do
	cd $(dirname "$line") && data=$(git --no-pager log -n 1 --format=format:"%H,%cd,%an <%ce>" HEAD)
	IFS=","
	for x in $data
	do
		echo $x >> .gitcommit
	done
done

# kill .git dir
for i in ".git*" ".hg*" ".bzr*" ".svn*"
do
	find $VENDOR -type d -name "$i" -print0 | xargs -0 rm -rf
done
