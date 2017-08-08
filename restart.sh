#!/bin/bash
buildEnv=prod
workDir=$(pwd)
while getopts ":d" opt; do
  case ${opt} in
    d ) buildEnv=dev
      ;;
  esac
done
mapSrc=''
if [ $buildEnv == 'dev' ]; then
    mapSrc="-v $workDir:/go/src/github.com/dangilk/bggscraper"
fi
docker stop bggService
docker stop bggScraper
docker rm bggService
docker rm bggScraper
docker run -v /private/var:/root/work ${mapSrc} -d --net bggnetwork --name bggScraper --restart unless-stopped golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggScraper"
docker run -v /private/var:/root/work ${mapSrc} -d -p 9090:9090 --restart unless-stopped --net bggnetwork --name bggService golang sh -c "go get github.com/dangilk/bggscraper/... && exec bggscraper bggService"
