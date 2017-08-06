docker stop bggService
docker stop bggScraper
docker rm bggService
docker rm bggScraper
docker run -v /private/var:/root/work -d --net bggnetwork --name bggScraper --restart unless-stopped golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggScraper"
docker run -v /private/var:/root/work -d -p 9090:9090 --restart unless-stopped --net bggnetwork --name bggService golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggService"
