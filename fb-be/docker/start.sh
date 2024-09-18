#!/bin/sh
# Check if FileBot license file exists and copy it to the correct location
if [ -f "$FILEBOT_LICENSE_PATH" ]; then
    echo "Copying FileBot license file from $FILEBOT_LICENSE_PATH to /opt/filebot/.license/license.psm"
    mkdir -p /opt/filebot/.license
    cp "$FILEBOT_LICENSE_PATH" /opt/filebot/.license/license.psm
else
    echo "FileBot license file not found at $FILEBOT_LICENSE_PATH, continuing without license."
fi

# Run the CMD as passed in from the Dockerfile
exec "$@"
