TILT_BIN=$(command -v tilt)
BIN_DIR=$PWD/temp/tilt/bin

function install_tilt {
  if [[ -z "$TILT_BIN" ]]; then
    echo "Tilt binary not found."
    echo "Intalling Tilt."
    curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash
  fi
} 

function test_setup {
  if ! $BIN_DIR/kubectl cluster-info --context kind-kpng-proxy 1>/dev/null 2>/dev/null ; then
    echo "Tilt setup not found."
    echo "Please run 'make tilt-setup' before trying tilt-up or tilt-down."
    exit 1
  fi
}

install_tilt
test_setup