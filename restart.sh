#kill $(ps aux | grep '[b]ggService' | awk '{print $2}')
#kill $(ps aux | grep '[b]ggScraper' | awk '{print $2}')
#nohup go run *.go bggService &
#nohup go run *.go bggScraper &
# ^ that stuff is old, i think this will work...
systemctl restart bgg-scraper
systemctl restart bgg-service
