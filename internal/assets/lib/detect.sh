#!/usr/bin/env bash
# OS detection. Sets OS_FAMILY and OS_ID.
# Source this before sourcing any os/*.sh module.

detect_os() {
  if [ ! -f /etc/os-release ]; then
    echo "ERROR: /etc/os-release not found. Unsupported OS." >&2
    exit 1
  fi

  . /etc/os-release
  OS_ID="${ID:-unknown}"
  OS_VERSION="${VERSION_ID:-unknown}"

  case "$OS_ID" in
    ubuntu|debian)
      OS_FAMILY="debian"
      ;;
    fedora|rhel|centos|rocky|almalinux|ol)
      OS_FAMILY="fedora"
      ;;
    arch|manjaro)
      OS_FAMILY="arch"
      ;;
    alpine)
      OS_FAMILY="alpine"
      ;;
    *)
      echo "ERROR: Unsupported OS: $OS_ID ($OS_VERSION)" >&2
      echo "Supported: Debian, Ubuntu, Fedora, RHEL, CentOS, Rocky, Alma, Arch, Manjaro, Alpine" >&2
      exit 1
      ;;
  esac

  echo "[detect] OS: $OS_ID $OS_VERSION (family: $OS_FAMILY)"
}

# Auto-detect if sourced
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  detect_os
  echo "OS_FAMILY=$OS_FAMILY"
  echo "OS_ID=$OS_ID"
  echo "OS_VERSION=$OS_VERSION"
fi