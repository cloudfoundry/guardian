RED=1
GREEN=2
function print_message {
  message=$1
  colour=$2
  printf "\r\033[00;3${colour}m[${message}]\033[0m\n"
}
