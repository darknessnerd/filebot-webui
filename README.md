# filebot-webui


To pass the FileBot license file during runtime, you need to set the environment variable FILEBOT_LICENSE_PATH to point 
to the location of the license file on your host system when you run the Docker container.

Here is an example Docker command that runs the container with the license file provided via an environment variable:

Make sure to mount the correct path for the license file when running the container:

bash
Copia codice
docker run -d \
-e FILEBOT_LICENSE_PATH="/config/license.psm" \
-v /path/on/host/to/filebot/config:/config \
-p 3001:3001 \
your-image-name
Ensure that /path/on/host/to/filebot/config/license.psm on your host machine actually contains the license file.

-
Replace your-image-name with the name or tag of your Docker image. This command will start the container with 
the FileBot license file applied at runtime.
