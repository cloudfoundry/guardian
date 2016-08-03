function show_groot_banner {
  cat $(dirname $BASH_SOURCE)/groot.ascii
  echo
  echo "I AM $(whoami)"
  echo
}
