# filebot-webui


To pass the FileBot license file during runtime, you need to set the environment variable FILEBOT_LICENSE_PATH to point 
to the location of the license file on your host system when you run the Docker container.

Here is an example Docker command that runs the container with the license file provided via an environment variable:

Make sure to mount the correct path for the license file when running the container:

bash
Copia codice
docker run  \
-e FILEBOT_LICENSE_PATH="/config/fileb.psm" \
-e INPUT_DIRS="/downloads" \
-e OUTPUT_DIRS="/movies, /tv_shows" \
-v /media/media4/home/filebot:/config \
-v /media/media0/deluge/donwloads:/downloads \
-v /media/media4/movies:/movies \
-v /media/media4/tv_shows:/tv_shows \
-p 8088:80 \
darknessnerd/filebot-webui
Ensure that /path/on/host/to/filebot/config/license.psm on your host machine actually contains the license file.

-
Replace your-image-name with the name or tag of your Docker image. This command will start the container with 
the FileBot license file applied at runtime.
docker run -d \
-p 5800:5800 \
-v /host/input1:/input1 \
-v /host/input2:/input2 \
-v /host/output1:/output1 \
-v /host/output2:/output2 \
darknessnerd/filebot-webui
