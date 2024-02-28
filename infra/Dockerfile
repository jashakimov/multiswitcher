FROM ubuntu
# Установка необходимых пакетов
RUN apt-get update && \
    apt-get install -y \
    wget \
    net-tools \
    libpcap-dev \
    iproute2 \
    iputils-ping \
    gcc \
    g++ \
    zlib1g-dev \
    libmpc-dev \
    libmpfr-dev \
    libgmp-dev \
    && rm -rf /var/lib/apt/lists/*

# Установка Golang 1.20
RUN wget https://golang.org/dl/go1.20.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz && \
    rm go1.20.linux-amd64.tar.gz

# Установка переменных окружения для Golang
EXPOSE 9000
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"
ENV DOCKER_DEFAULT_PLATFORM=linux/amd64