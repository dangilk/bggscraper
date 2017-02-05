kill $(ps aux | grep '[g]o run main.go' | awk '{print $2}')
nohup go run *.go service &
nohup go run *.go scraper &
