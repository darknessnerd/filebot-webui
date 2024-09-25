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
-e OUTPUT_DIRS="/media" \
-v /media/media4/home/filebot:/config \
-v /media/media0/deluge/donwloads:/downloads \
-v /media/media4/movies:/media/movies \
-v /media/media4/tv_shows:/media/tv_shows \
-p 8088:80 \
darknessnerd/filebot-webui
Ensure that path in the host /media/media4/home/filebot contain the file fileb.psm declared in this env FILEBOT_LICENSE_PATH  on your host machine actually contains the license file.

-

