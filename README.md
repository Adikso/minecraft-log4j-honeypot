# Minecraft Log4j Honeypot

This honeypots runs fake Minecraft server (1.7.2 - 1.16.5 without snapshots) waiting to be exploited. Payload classes are saved to `payloads/` directory.

## Requirements
- Golang 1.16+

## Running

### Natively
```
git clone https://github.com/Adikso/minecraft-log4j-honeypot.git
cd minecraft-log4j-honeypot
go build .
./minecraft-log4j-honeypot
```

### Using docker
```
mkdir payloads
docker run --rm -it --mount type=bind,source="${PWD}/payloads",target=/payloads --user=`id -u` -p 25565:25565 adikso/minecraft-log4j-honeypot:latest
```
