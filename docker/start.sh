#!/bin/sh
set -eu

if ! command -v filebot > /dev/null 2>&1; then
    echo "FileBot not found in PATH"
    exit 1
fi

if [ -f "$FILEBOT_LICENSE_PATH" ]; then
    echo "Applying FileBot license from $FILEBOT_LICENSE_PATH"
    mkdir -p /opt/filebot/.license
    cp "$FILEBOT_LICENSE_PATH" /opt/filebot/.license/license.psm
    filebot --license /opt/filebot/.license/license.psm || {
        echo "Failed to apply FileBot license"
        exit 1
    }
else
    echo "No FileBot license at $FILEBOT_LICENSE_PATH — continuing without license"
fi

exec "$@"
