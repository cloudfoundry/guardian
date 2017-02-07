#!/bin/bash

cp -r grootfs grootfs-updated
cd grootfs-updated
./script/deps -u  github.com/containers/image

# Commit if changes

if [[ `git diff --cached` ]]; then
  git config --global user.email "cf-garden+garden-gnome@pivotal.io"
  git config --global user.name "I am Groot CI"
  grootfs_changes=$(git diff --cached | tail -n +2)
  git commit -m "$(printf "Bump dependencies\n\n${grootfs_changes}")"
else
  echo "No changes"
fi
