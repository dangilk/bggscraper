# bggscraper
This is my fun little project.

##Docker
mysql should be set up within docker. use this: docker run -p 3307:3306 --name mysqldb -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7
Connect locally like this: mysql -h 127.0.0.1 -P 3307 -u root -p

#Setup
add service files for systemd (probably in /lib/systemd/system). There are some basic examples included in the repo.
Enable the services with `systemctl enable bgg-scraper`, start them with `systemctl start bgg-scraper`

Setting up inside docker:
set up a container network:
`docker network create testnetwork`

put a local password file on the local host e.g.
`/private/var/mysqlpw.txt` -> `root`

setup mysql container: 
`docker run --net testnetwork --name mysqldb -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7` * make sure to actually create the DB after this!

setup go container. map pw file to container, start on our network, clean up container after running, then pull down from github and build/run
`docker run -v /private/var:/root/work --net testnetwork --rm golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggScraper"`