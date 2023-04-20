#!/bin/bash

# ```sh
# curl -sSf https://raw.githubusercontent.com/pirakansa/ppkgmgr/develop/scripts/uninstall.sh | sudo bash -
# ```

PROJECT_NAME="ppkgmgr"
INSTALL_DIR="/usr/bin"
CONFIGURE_DIR="/etc/${PROJECT_NAME}"
CONFIGURES_DIR="${CONFIGURE_DIR}/pkg.d"
CACHE_DIR="/var/cache/${PROJECT_NAME}"

rm -f  "${INSTALL_DIR}/${PROJECT_NAME}"
rm -fr "${CONFIGURE_DIR}"
rm -fr "${CACHE_DIR}"

echo "uninstalled"
