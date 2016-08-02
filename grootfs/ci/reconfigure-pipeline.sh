#!/bin/bash -ex

cd $(dirname $0)/..

if [ -z $1 ]; then
  echo "No target passed, using 'grootfs-ci'"
  FLYRC_TARGET="grootfs-ci"
else
  FLYRC_TARGET=$1
fi

check_fly_alias_exists() {
  set +e
  grep $FLYRC_TARGET ~/.flyrc > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "Please ensure $FLYRC_TARGET exists in ~/.flyrc and that you have run fly login"
    exit 1
  fi
  set -e
}

check_fly_alias_exists

fly --target="$FLYRC_TARGET" set-pipeline --pipeline=grootfs --config=ci/pipeline.yml --load-vars-from=/Users/pivotal/workspace/grootfs-ci-secrets/vars/aws.yml
