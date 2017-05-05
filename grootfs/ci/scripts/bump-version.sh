#!/bin/bash
set -e -x

OUTPUT_PATH=$PWD/bumped-grootfs-repo
VERSION=$(cat grootfs-next-final-version/number)

cd grootfs-git-repo

sed -i -e "s/const version.*/const version = \"${VERSION}\"/" version.go
git add version.go

if [[ `git diff --cached` ]]; then
	git config --global user.email "cf-garden+garden-gnome@pivotal.io"
	git config --global user.name "I am Groot CI"
	git commit -m "Bump binary version to ${VERSION}"
fi

cp -r . $OUTPUT_PATH
