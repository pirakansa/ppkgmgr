###### Dockerfile
FROM golang:1.24-bullseye

###### Env 
ARG USER=builder
ARG UID=1000

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LANGUAGE=en_US:en \
    LC_ALL=C.UTF-8

###### Run
RUN apt update -y && \
    apt install -y busybox gosu ; \
    useradd -u ${UID} -s /bin/bash -m ${USER} && \
    usermod -G root ${USER} && \
    install --owner=${USER} --directory /work && \
    install --owner=${USER} --directory /.cache && \
    chmod +s /usr/sbin/gosu ; \
    \
    rm -rf /var/lib/apt/lists/* ; \
    rm -rf /var/cache/apt/archives/* ; \
    rm -rf /var/cache/apt/*.bin ; \
    rm -rf /var/cache/debconf/*-old ; \
    rm -rf /var/lib/dpkg/*-old ; \
    rm -rf /var/log/* ; \
    rm -rf /var/tmp/* ; \
    rm -rf /tmp/*

###### login user
USER ${USER}

###### working directory
WORKDIR /work

CMD ["/bin/bash"]


