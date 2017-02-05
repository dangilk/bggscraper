kill $(ps aux | grep '[b]ggService' | awk '{print $2}')
kill $(ps aux | grep '[b]ggScraper' | awk '{print $2}')
nohup go run *.go bggService &
nohup go run *.go bggScraper &
