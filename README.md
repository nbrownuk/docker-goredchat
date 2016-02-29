# goredchat
goredchat is a simple chat application written in Golang by [Dj Walker-Morgan](https://www.compose.io/articles/redis-go-and-how-to-build-a-chat-application/). The [original version](https://github.com/compose-ex/goredchat) has been altered cosmetically, and is designed to run in a Docker container, against a Redis server (perhaps also running in a container). The purpose of dockerizing goredchat, is to demonstrate Docker's overlay networking capability.
## Usage
Run gorechat specifying a Redis server URL to connect to, along with a chat username:
```
Usage: goredchat [-r URL] username
  e.g. goredchat -r redis://redis_svr:6379 antirez

  If -r URL is not used, the REDIS_URL env must be set instead
```
## goredit Docker image
If you plan to run a Redis server on a previously created Docker overlay network, you might start the service with:
```
$ docker run -d --restart unless-stopped --net-alias redis_svr --net overlay_nw redis:alpine redis-server --appendonly yes
```
In order to chat, a goredchat container can be started with:
```
$ docker run -it --rm --net overlay_nw nbrown/goredchat -r redis://redis_svr:6379 antirez
```