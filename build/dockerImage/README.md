# docker image

## build

```sh
$ cd $project_root
$ SUDO_USER=$(/usr/bin/logname)
$ docker build \
    --build-arg UID=$(id -u $SUDO_USER) \
    -t golang-work:latest .devcontainer
```


## run

```sh
$ cd $project_root
$ docker container run \
    -i -t --rm \
    -v `pwd`:/work \
    golang-work:latest \
    make
```
