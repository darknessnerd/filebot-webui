# filebot-webui


To pass the FileBot license file during runtime, you need to set the environment variable FILEBOT_LICENSE_PATH to point to the location of the license file on your host system when you run the Docker container.

Here is an example Docker command that runs the container with the license file provided via an environment variable:

Command Example
bash
Copia codice
docker run -d \
-e FILEBOT_LICENSE_PATH="/path/on/host/to/license.psm" \
-v /path/on/host/to/config:/config \
-p 3001:3001 \
your-image-name
Explanation:
-e FILEBOT_LICENSE_PATH="/path/on/host/to/license.psm": This sets the environment variable FILEBOT_LICENSE_PATH inside the container, pointing to the location of the license file on your host system. 
For example, replace /path/on/host/to/license.psm with the actual path to your license file.

-v /path/on/host/to/config:/config: This mounts the /config directory from the host to the container, ensuring that the container has access to the license file (and any other configurations).

-p 3001:3001: This exposes port 3001 of the container and maps it to port 3001 on the host.

Replace your-image-name with the name or tag of your Docker image. This command will start the container with the FileBot license file applied at runtime.
