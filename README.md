# bggscraper
This is my fun little project.

## Docker
mysql should be set up within docker. use this: docker run -p 3307:3306 --name mysqldb -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7
Connect locally like this: mysql -h 127.0.0.1 -P 3307 -u root -p

# Setup (maybe dont use systemd anymore...)
add service files for systemd (probably in /lib/systemd/system). There are some basic examples included in the repo.
Enable the services with `systemctl enable bgg-scraper`, start them with `systemctl start bgg-scraper`

Setting up inside docker:
set up a container network:
`docker network create bggnetwork`

put a local password file on the local host e.g.
`/private/var/mysqlpw.txt` -> `root`

add some swap space for the mysql image. see these instructions:
https://www.digitalocean.com/community/tutorials/how-to-configure-virtual-memory-swap-file-on-a-vps

setup mysql container: 
`docker run --net bggnetwork --name mysqldb -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=hello -d --restart unless-stopped mysql:8.0.2` * make sure to set the root password to the real password (not "root")

setup go container. map pw file to container, detach container, start on our network, set the restart policy, then pull down from github and build/run
`docker run -v /private/var:/root/work -d --net bggnetwork --name bggScraper --restart unless-stopped golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggScraper"`

similarly, start the http service. note that we expose port 9090 for incoming traffic: `docker run -v /private/var:/root/work -d -p 9090:9090 --restart unless-stopped --net bggnetwork --name bggService golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggService"`
