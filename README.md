---

# 📂 FileBot-WebUI 🖥️

🎉 Welcome to **FileBot-WebUI**, where managing your files has never been this much fun (or this automated)! 🎉

---

## 🚀 What is this?

FileBot-WebUI is your go-to tool for renaming and organizing media files (like TV shows, movies, and more) with the power of **FileBot**, served to you through a beautiful web interface.

Think of it like a personal assistant for your media files, but with way fewer coffee breaks ☕.

---

## 🎬 Features

- 🌐 **Web-based Interface**: Manage your media files directly from your browser! No need to fiddle with those pesky command lines.
- 🖥️ **Docker-ready**: Run everything inside a cozy little Docker container. So easy, even your cat can do it 🐱.
- ⚙️ **Environment Variables**: Customize your setup with variables because who doesn't love a little extra control? 💪
- 📝 **Automatic Media Renaming**: Name your files so cleanly, your friends will think you're Marie Kondo'ing your media collection.

---

## 🛠️ How to Set it Up

1. **Grab your License** 🏷️:
   Make sure you've got your FileBot license file handy. You'll need to pass it when you run the container.

2. **Run it** 🏃:
   Use this Docker command, but don't forget to point to the right license file! 🎯

   ```bash
   docker run \
   -e FILEBOT_LICENSE_PATH="/config/fileb.psm" \
   -e INPUT_DIRS="/downloads" \
   -e OUTPUT_DIRS="/media" \
   -v /media/media4/home/filebot:/config \
   -v /media/media0/deluge/downloads:/downloads \
   -v /media/media4/movies:/media/movies \
   -v /media/media4/tv_shows:/media/tv_shows \
   -p 8088:80 \
   darknessnerd/filebot-webui
   ```

3. **🎉 Success!**  
   Now your media files can experience the life-changing magic of renaming with **FileBot-WebUI**. Just sit back and let the magic happen ✨.

---

## 📚 Documentation

- Set the `FILEBOT_LICENSE_PATH` environment variable to the path where your license file lives 🗃️. Example: `/config/fileb.psm`.
  - in this example the license is located in /media/media4/home/filebot/fileb.psm in the host and the ENV variable must be set with /config/fileb.psm
- Mount your input and output directories like a pro by mapping `/downloads` for input and `/media` for output 🗂️.
- Make sure the host paths are correct, especially for the license file. If that file's missing, things could go sideways fast 😅.

---

## 🎨 Tech Stack

- **Vue.js** 🖌️ - Because a slick UI makes everything better!
- **JavaScript** 🤖 - Gotta keep things interactive!
- **SCSS** 🎨 - For that extra bit of style.
- **Dockerfile** 🐳 - Keeping things containerized and tidy.
- **Shell scripting** 🖥️ - For those command-line ninjas.

---

## 😎 Want to Contribute?

Feel free to fork the repo (we promise, it won’t fork you back 🍴) or open a pull request. Contributions, bug fixes, and fun suggestions are always welcome!

### tag 
Example of Git Tagging:
To trigger a build with SemVer, you would tag your commit:

bash
Copia codice
git tag v1.0.0
git push origin v1.0.0
This will ensure the Docker image gets tagged as 1.0.0 when pushed to Docker Hub.
