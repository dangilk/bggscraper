# bggscraper
This is my fun little project.

##Docker
mysql should be set up within docker. use this: docker run -p 3307:3306 --name mysqldb -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7
Connect locally like this: mysql -h 127.0.0.1 -P 3307 -u root -p

##Setup
add service files for systemd (probably in /lib/systemd/system). There are some basic examples included in the repo.
Enable the services with `systemctl enable bgg-scraper`, start them with `systemctl start bgg-scraper`
