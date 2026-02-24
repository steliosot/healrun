#!/bin/bash

healrun_install() {
  local arch=$(uname -m)
  local os=$(uname -s | tr '[:upper:]' '[:lower:]')

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch"; exit 1 ;;
  esac

  case "$os" in
    darwin) os="darwin" ;;
    linux) os="linux" ;;
    *) echo "Unsupported OS: $os"; exit 1 ;;
  esac

  local binary_name="healrun-${os}-${arch}"
  local url="https://github.com/steliosot/healrun/releases/latest/download/${binary_name}"

  echo "Installing healrun for ${os}-${arch}..."

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o /tmp/healrun || {
      echo "Download failed, trying build from source..."
      return 1
    }
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O /tmp/healrun || {
      echo "Download failed, trying build from source..."
      return 1
    }
  else
    echo "curl or wget required"
    exit 1
  fi

  chmod +x /tmp/healrun
  sudo mv /tmp/healrun /usr/local/bin/healrun

  if [ $? -eq 0 ]; then
    echo "healrun installed to /usr/local/bin/healrun"
    echo "Create config: mkdir -p ~/.healrun && cp config.example.yaml ~/.healrun/config.yaml"
  else
    echo "Install failed"
    exit 1
  fi
}

healrun_install || {
  echo "Falling back to build from source..."
  if command -v go >/dev/null 2>&1; then
    git clone https://github.com/steliosot/healrun.git /tmp/healrun-build
    cd /tmp/healrun-build
    go build -ldflags="-s -w" -o healrun ./cmd/healrun
    sudo cp healrun /usr/local/bin/
    rm -rf /tmp/healrun-build
    echo "healrun built and installed"
  else
    echo "Go not found. Install Go or download manually from:"
    echo "https://github.com/steliosot/healrun/releases"
    exit 1
  fi
}