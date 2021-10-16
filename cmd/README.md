# Instructions for compiling

On linux, you'll need to manually install netlink, possibly:
```
sudo apt-get install libnl-genl-3-dev
```

Then you can run:

```
GOOS=linux go build -o kpng ./kpng
```

