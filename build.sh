#!/usr/bin/env bash

function get_os() {
  unameOut="$(uname -s)"
  case "${unameOut}" in
  Linux*)
    echo -n "linux"
    ;;
  Darwin*)
    echo -n "macos"
    ;;
  CYGWIN*)
    echo -n "cygwin"
    ;;
  MINGW*)
    echo -n "mingw"
    ;;
  *)
    echo "Cannot detect your operating system.  Exiting."
    exit 1
    ;;
  esac
}

os="$(get_os)"

function check_available() {
  which $1 >/dev/null
  if [ $? -ne 0 ]; then
    echo "**** ERROR needed program missing: $1"
    exit 1
  fi
}

check_available 'which'
check_available 'realpath'
check_available 'dirname'

bashver="${BASH_VERSION:0:1}"

if ! [[ "$bashver" =~ ^[5-9]$ ]]; then
  echo 'You need MOAR bash-fu!  Your version of `bash` is waaaay too old!  What are you running?  Commodore 64?'
  echo "Your bash version is: ${BASH_VERSION}"
  echo "Your need at least a bash version of 5 or higher"
  echo
  case "${os}" in
  linux)
    echo "Thank God, you're running Linux.  There's hope."
    echo "Use your package manager to upgrade bash."
    echo "If your Linux distribution can't get a recent version of bash, change distros."
    ;;
  macos)
    echo 'MacOS: likely `brew install bash` will be your friend.'
    ;;
  cygwin)
    echo "Uhhhh ... CygWin.  Not sure how to help here."
    ;;
  mingw)
    echo "Uhhhh ... MinGW.  Not sure how to help here."
    ;;
  *)
    echo "I have no idea."
    echo "Repent sins."
    ;;
  esac
  exit 1
fi

script_dir=$(dirname "$(realpath -e "$0")")
cwd="$(echo "$(pwd)")"
function cleanup() {
  cd "$cwd"
}
# Make sure that we get the user back to where they started
trap cleanup EXIT

# This is necessary because we reference things relative to the script directory
cd "$script_dir"

function usage() {
  echo "Usage: build.sh [-h|--help] [-c|--clean] [-C|--clean-all] [-b|--build]"
  echo
  echo '    Build transformer.'
  echo
  echo "Arguments:"
  echo "  -h|--help                      This help text"
  echo '  -c|--clean                     Clean generated artifacts.'
  echo "  -C|--clean-all                 Clean all the artifacts and the Go module cache."
  echo "  -b|--build                     Build 'transformer' using local tooling"
  echo "  -D|--dist                      Create a tarball distribution of 'transformer'"
}

clean=0
clean_all=0
build=0
create_dist=0

while [[ $# -gt 0 ]]; do
  key="$1"

  case $key in
  -h | --help)
    usage
    exit 0
    ;;
  -c | --clean)
    clean=true
    shift
    ;;
  -C | --clean-all)
    clean_all=true
    shift
    ;;
  -b | --build)
    build=true
    shift
    ;;
  -D | --dist)
    create_dist=true
    shift
    ;;
  *)
    echo "ERROR: unknown argument $1"
    echo
    usage
    exit 1
    ;;
  esac
done

dist="dist"
dexec="${dist}/exec"
dist_out="dist-out"

function go_cmd () {
  out="$1"
  envvar="$2"
  eval "$envvar CGO_ENABLED=0 go build -v -o '$out' ."
}

function linux_chown () {
  sudo chown --changes --silent --no-dereference --recursive "${uid}:${gid}" "$1"
}

function macos_chown () {
  sudo chown -P -R "${uid}:${gid}" "$1"
}

function linux_tar () {
  tar cvzf transformer.tar.gz -C ../dist --transform 's,^\.,sengen,' .
}

function macos_tar () {
  tar -cvzf transformer.tar.gz -C ../dist -s '/^\./sengen/' .
}

tar_cmd="linux_tar"
chown_cmd="linux_chown"
if [ "$os" = "macos" ]; then
  chown_cmd="macos_chown"
  tar_cmd="macos_tar"
fi

go_used="Building with local go: $(which go)"
build_cmd="go_cmd"

if [ "$clean_all" = true ]; then
  echo "Deep cleaning..."
  clean=true
  go clean --modcache
fi

if [ "$clean" = true ]; then
  echo "Regular cleaning..."
	rm -fr ./dist/exec/transformer-*
	rm -fr "./${dist_out}"
	rm -f transformer
	go clean .
fi

if [ "$create_dist" = true ]; then
  echo "Creating distribution..."
  echo "$go_used"
	echo "Compiling for MacOS"
	$build_cmd "${dexec}/transformer-darwin-amd64" 'GOOS=darwin GOARCH=amd64'
  $chown_cmd "${dexec}/transformer-darwin-amd64"
	echo "Compiling for Linux"
	$build_cmd "${dexec}/transformer-linux-amd64" 'GOOS=linux GOARCH=amd64'
  $chown_cmd "${dexec}/transformer-linux-amd64"
	echo "Compiling for Windows"
	$build_cmd "${dexec}/transformer-windows-amd64.exe" 'GOOS=windows GOARCH=amd64'
  $chown_cmd "${dexec}/transformer-windows-amd64.exe"

	rm -fr "${dist_out}"
	mkdir -p "${dist_out}"
	cd "${dist_out}"; $tar_cmd
elif [ "$build" = true ]; then
  echo "Building..."
  echo "$go_used"
	$build_cmd "transformer" || exit 10
fi
