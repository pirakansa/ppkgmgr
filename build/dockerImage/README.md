# docker image

## build

```sh
$ cd $project_root
$ docker build \
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
