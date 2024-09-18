#!/bin/sh

# Set strict mode for debugging
set -e   # Exit immediately if any command fails
set -u   # Treat unset variables as an error
set -x   # Enable debugging mode to print commands being executed

# Ensure the filebot binary is available
if ! command -v filebot > /dev/null 2>&1; then
    echo "FileBot is not installed or not found in PATH"
    exit 1
fi

# Check if FileBot license file exists and copy it to the correct location
if [ -f "$FILEBOT_LICENSE_PATH" ]; then
    echo "Copying FileBot license file from $FILEBOT_LICENSE_PATH to /opt/filebot/.license/license.psm"
    mkdir -p /opt/filebot/.license
    cp "$FILEBOT_LICENSE_PATH" /opt/filebot/.license/license.psm

    # Apply the license
    echo "Applying FileBot license from /opt/filebot/.license/license.psm"
    filebot --license /opt/filebot/.license/license.psm || {
        echo "Failed to apply FileBot license."
        exit 1
    }
else
    echo "FileBot license file not found at $FILEBOT_LICENSE_PATH, continuing without applying license."
fi

# Run the CMD passed from Dockerfile
exec "$@"
