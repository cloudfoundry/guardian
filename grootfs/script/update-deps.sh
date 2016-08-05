#!/bin/bash
set -e

function main {
  check_clean_status
  remove_existing_submodules
  glide install
  add_required_submodules
  commit_warning
}

RED=1
GREEN=2
function print_message {
  message=$1
  colour=$2
  printf "\r\033[00;3${colour}m[${message}]\033[0m\n"
}

function check_clean_status {
  print_message "CHECKING IF STATUS IS CLEAN" $GREEN
  if ! [ -z "$(git status --porcelain)" ]; then
    print_message "STATUS NOT CLEAN, COMMIT FIRST" $RED
    exit 1
  else
    print_message "STATUS CLEAN, wait for it ... GROOT TO GO!" $GREEN
  fi
}

function remove_existing_submodules {
  if [ -d vendor/ ]; then
    print_message "REMOVING EXISTING SUBMODULES FROM VENDOR" $GREEN
    rm -rf vendor/
    git rm -rf vendor/ &> /dev/null
    git submodule deinit --all -f
  fi
}

function convert_url {
  repoPath=$1
  url_conversion_rules=("s/code.cloudfoundry.org/github.com\/cloudfoundry/" "s/golang.org\/x/go.googlesource.com/")

  url="https://"$(echo $repoPath | sed -e 's/.\/vendor\///')
  for rule in ${url_conversion_rules[@]}; do
    url=$(echo $url | sed $rule)
  done

  echo $url
}

function add_submodule {
  path=$1
  repoPath=$(echo $path | sed -e 's/\/.git//')
  url=$(convert_url $repoPath)

  git submodule add $url $repoPath &> /dev/null
}

function add_required_submodules {
  print_message "ADDING REQUIRED SUBMODULES" $GREEN
  requires_submodules=$(find . -name ".git" -mindepth 2)
  for submodule in ${requires_submodules[@]}; do
    print_message "ADDING SUBMODULE $submodule" $GREEN

    add_submodule $submodule
  done
}

function commit_warning {
	echo "Dependencies updated. Commit now."
}

main
