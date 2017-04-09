package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"io/ioutil"
	"encoding/xml"
	"fmt"
	"time"
	"os"
	"bufio"
	"strconv"
	"github.com/muesli/regommend"
	"encoding/json"
	"math"
	"strings"
)

var sqlDb *sql.DB
const baseUrlApi2 = "https://www.boardgamegeek.com/xmlapi2"
const baseUrlApi1 = "https://www.boardgamegeek.com/xmlapi"
var exploredUsers map[int]bool
var collectionInsertStmt *sql.Stmt
var gameMetaInsertStmt *sql.Stmt
var currentForumListId int
var operatingMode string

func main() {
	openDb()
	setupDb()

	operatingMode = os.Args[1]
	if operatingMode == "bggScraper" {
		log.Println("starting scraper")
		startScraperService()
	} else if operatingMode == "bggService" {
		log.Println("starting query service")
		startQueryService()
	} else {
		log.Println("no commands found, shutting down")
	}



	//closeDb()
}

// SERVICE SECTION
func topSuggestions(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()  // parse arguments, you have to call this by yourself
	userName := r.FormValue("userName")
	userUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", userName)
	if (!isUserInDb(userName)) {
		getXml(userUrl, createUserProcessor(false))
	}

	fmt.Fprint(w, "what would I recommend for ", userName, "...\n")
	for _, rec := range recommend(userName) {
		fmt.Fprint(w, "I would recommend: ", rec.Name, "\n")
	}
}

func startQueryService() {
	http.HandleFunc("/topSuggestions", topSuggestions) // set router
	err := http.ListenAndServe(":9090", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// SCRAPER SECTION

func startScraperService() {
	currentForumListId = getCurrentForumList()
	for {
		logToFile("start scraper iteration for forumList: " + strconv.Itoa(currentForumListId))
		exploredUsers = make(map[int]bool)
		getUsersFromForumList(currentForumListId)
		currentForumListId++
		updateCurrentForumList(currentForumListId)
	}
}

func updateCurrentForumList(forumId int) {
	_, err := sqlDb.Exec("REPLACE INTO current_forumlist(id, forumId) VALUES(?,?)", 1, forumId)
	if err != nil {
		logToFile(err)
	}
}

func getCurrentForumList() int {
	ret := 0;
	var (
		forumId int
	)
	rows, err := sqlDb.Query("select forumId from current_forumlist where id = 1")
	if err != nil {
		logToFile(err)
		return getCurrentForumList()
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&forumId)
		if err != nil {
			logToFile(err)
			return getCurrentForumList()
		}
		ret = forumId
	}
	err = rows.Err()
	if err != nil {
		logToFile(err)
		return getCurrentForumList()
	}
	return ret
}

// get a forumlist, then get its forums, then get its threads, then get its articles, and finally get users from articles
func getUsersFromForumList(forumId int) {
	logToFile("processing forumlist with id: " + strconv.Itoa(forumId))
	forumListUrl := fmt.Sprintf(baseUrlApi2 + "/forumlist?id=%d&type=thing", forumId)
	getXml(forumListUrl, processForumList)
}

func processForumList(bytes []byte) {
	var forumList ForumList
	err := xml.Unmarshal(bytes, &forumList)
	if err != nil {
		logToFile("error unmarshalling forumlist xml, aborting")
		return
	}
	for _,forum := range forumList.Forums {
		forumUrl := fmt.Sprintf(baseUrlApi2 + "/forum?id=%d", forum.Id)
		getXml(forumUrl, processForum)
	}
}

func processForum(bytes []byte) {
	var forum Forum
	err := xml.Unmarshal(bytes, &forum)
	if err != nil {
		logToFile("error unmarshalling forum xml, aborting")
		return
	}
	if forum.Id < 1 {
		// reset forum list
		currentForumListId = 0
	}
	for _,thread := range forum.Threads.Threads {
		threadUrl := fmt.Sprintf(baseUrlApi2 + "/thread?id=%d", thread.Id)
		getXml(threadUrl, processThread)
	}
}

func processThread(bytes []byte) {
	var thread Thread
	err := xml.Unmarshal(bytes, &thread)
	if err != nil {
		logToFile("error unmarshalling thread xml, aborting")
		return
	}
	for _,article := range thread.Articles.Articles {
		userUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", article.Author)
		getXml(userUrl, createUserProcessor(true))
	}
}

func createUserProcessor(exploreBuddies bool) XmlProcessor {
	return func (bytes []byte) {
		var user User
		err := xml.Unmarshal(bytes, &user)
		if err != nil {
			logToFile("error unmarshalling user xml, aborting")
			return
		}
		if len(user.Name) < 1 {
			logToFile("skipping empty user")
			return
		}
		if exploredUsers != nil {
			if _, exists := exploredUsers[user.Id]; exists {
				return
			} else {
				exploredUsers[user.Id] = true
			}
		}

		// get the users collection
		logToFile("process user: " + user.Name)
		collectionUrl := fmt.Sprintf(baseUrlApi1 + "/collection/%s", user.Name)
		getXml(collectionUrl, createCollectionProcessor(user))


		// explore user friends
		if (exploreBuddies) {
			for _, buddy := range user.Buddies.Buddies {
				logToFile("get buddies xml")
				buddyUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", buddy.Name)
				getXml(buddyUrl, createUserProcessor(true))
			}
		}
	}
}



func createCollectionProcessor(user User) XmlProcessor {
	return func (bytes []byte) {
		var collectionItems CollectionItems
		err := xml.Unmarshal(bytes, &collectionItems)
		if err != nil {
			logToFile("error unmarshalling collection xml, aborting")
			return
		}
		userRatings := make(map[string]int)
		for _,item := range collectionItems.Items {
			insertCollection(user, item)
			userRatings[strconv.Itoa(item.ObjectId)] = item.Stats.Rating.Value
		}
		insertUserRatings(user.Name, strconv.Itoa(user.Id), userRatings)
	}
}

type XmlProcessor func(bytes []byte)

func getXml(url string, processor XmlProcessor) {
	// throttle requests a little
	time.Sleep(5 * time.Second)
	response, err := http.Get(url)
	if err != nil {
		retryGetXml(err, "error getting response - waiting for retry", url, processor, 30)
		return
	} else {
		defer response.Body.Close()
		statusCode := response.StatusCode
		if statusCode == 200 {
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				retryGetXml(err, "error reading response - waiting for retry", url, processor, 30)
			} else {
				processor(body)
			}
		} else if statusCode == 202 {
			retryGetXml(err, "received 202 - waiting for retry", url, processor, 5)
		} else if statusCode == 400 {
			logToFile("received error 400 - aborting")
		} else {
			retryGetXml(err, fmt.Sprintf("server error %d - waiting for retry", statusCode), url, processor, 30)
		}
	}
}

func retryGetXml(err error, retryMsg string, url string, processor XmlProcessor, sleepSeconds int) {
	if err != nil {
		logToFile(err, retryMsg)
	}
	time.Sleep(time.Duration(sleepSeconds) * time.Second)
	getXml(url, processor)
}
func openDb() {
	userPw := "root"
	file, err := os.Open("/root/work/mysqlpw.txt")
	if err == nil {
		// Create a new Scanner for the file.
		scanner := bufio.NewScanner(file)
		// Loop over all lines in the file and print them.
		for scanner.Scan() {
			line := scanner.Text()
			userPw += ":" + line
		}
	}
	open := userPw + "@tcp(127.0.0.1:3306)/hello"
	println(open)

	db, err := sql.Open("mysql", open)
	if err != nil {
		logToFile(err)
		panic("could not open db")
	}

	err = db.Ping()
	if err != nil {
		// do something here
	}
	sqlDb = db
}

func closeDb() {
	sqlDb.Close()
}

func insertCollection(user User, collection CollectionItem) {
	_, err := collectionInsertStmt.Exec(collection.Id, user.Id, user.Name, collection.Name, collection.NumPlays,
	collection.Status.Own, collection.Status.PrevOwned, collection.Status.ForTrade, collection.Status.Want,
	collection.Status.WantToPlay, collection.Status.WantToBuy, collection.Status.WishList, collection.Status.WishListPriority,
	collection.Status.PreOrdered, collection.Status.LastModified,
	collection.Stats.Rating.Value)
	if err != nil {
		logToFile(err)
		return
	}
	// update metadata
	_, err = gameMetaInsertStmt.Exec(collection.ObjectId, collection.Name, collection.YearPublished, collection.SubType,
		collection.Stats.MinPlayers, collection.Stats.MaxPlayers,
		collection.Stats.MinPlaytime, collection.Stats.MaxPlaytime, collection.Stats.PlayingTime, collection.Stats.NumOwned,
		collection.Stats.Rating.UsersRated.Value, collection.Stats.Rating.AverageRating.Value,
		collection.Stats.Rating.BayesAverageRating.Value, collection.Stats.Rating.StdDevRating.Value, collection.Stats.Rating.MedianRating.Value)
	if err != nil {
		logToFile(err)
		return
	}
}

func setupDb() {
	_, err := sqlDb.Exec("CREATE TABLE IF NOT EXISTS user_collections(" +
		"id INT NOT NULL PRIMARY KEY, " +
		"userId INT NOT NULL, " +
		"userName VARCHAR(200) NOT NULL, " +
		"gameName VARCHAR(1000) NOT NULL, " +
		"numPlays INT, " +
		"own BOOL, " +
		"prevOwned BOOL, " +
		"forTrade BOOL, " +
		"want BOOL, " +
		"wantToPlay BOOL, " +
		"wantToBuy BOOL, " +
		"wishList BOOL, " +
		"wishListPriority INT, " +
		"preOrdered BOOL, " +
		"lastModified VARCHAR(100), " +
		"userRating DOUBLE);")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS current_forumlist(id INT NOT NULL PRIMARY KEY, forumId INT NOT NULL);")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS user_ratings(userName VARCHAR(200) NOT NULL PRIMARY KEY, userId INT NOT NULL, ratingsJson LONGTEXT);")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS game_metadata(" +
		"id INT NOT NULL PRIMARY KEY, " +
		"gameName VARCHAR(1000) NOT NULL, " +
		"yearPublished INT, " +
		"subType VARCHAR(100), " +
		"minPlayers INT, " +
		"maxPlayers INT, " +
		"minPlaytime INT, " +
		"maxPlaytime INT, " +
		"playingTime INT, " +
		"numOwned INT, " +
		"ratingCount INT, " +
		"averageRating DOUBLE, " +
		"bayesAverageRating DOUBLE, " +
		"stdDevRating DOUBLE, " +
		"medianRating DOUBLE);")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}
	collectionInsertStmt, err = sqlDb.Prepare("REPLACE INTO user_collections(id, userId, userName, gameName," +
		"numPlays, own, prevOwned, forTrade, want, wantToPlay, wantToBuy, " +
		"wishList, wishListPriority, preOrdered, lastModified, " +
		"userRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}
	gameMetaInsertStmt, err = sqlDb.Prepare("REPLACE INTO game_metadata(id, gameName, yearPublished," +
		"subType, " +
		"minPlayers, maxPlayers, minPlaytime, maxPlaytime," +
		"playingTime, numOwned, ratingCount, averageRating, bayesAverageRating, stdDevRating, medianRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		logToFile(err)
		panic("could not create db")
	}
}

func insertUserRatings(userName string, userId string, ratings map[string]int) {
	log.Println(ratings)
	json, err := json.Marshal(ratings)
	if err != nil {
		logToFile(err)
		return
	}
	log.Println(json)
	_, err = sqlDb.Exec("REPLACE INTO user_ratings(userName, userId, ratingsJson) VALUES(?,?,?)",userName, userId, string(json))
	if err != nil {
		logToFile(err)
	}
}

func isUserInDb(userName string) bool {
	stmt, err := sqlDb.Prepare("select userName from user_ratings where userName = ? limit 1")
	if err != nil {
		logToFile(err)
		return false
	}
	defer stmt.Close()
	rows, err := stmt.Query(userName)
	if err != nil {
		logToFile(err)
		return false
	}
	ret := false
	for rows.Next() {
		ret = true
	}
	return ret
}

type UserRatingsBundle struct {
	userName string
	ratings map[string]int
}

func fetchUserRatingsSample() []UserRatingsBundle {
	resultSet := make([]UserRatingsBundle, 0)
	stmt, err := sqlDb.Prepare("select userName, ratingsJson from user_ratings limit 1000")
	if err != nil {
		logToFile(err)
		return resultSet
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		logToFile(err)
		return resultSet
	}
	defer rows.Close()
	return parseUserRatingsQuery(rows, resultSet)
}

func parseUserRatingsQuery(rows *sql.Rows, resultSet []UserRatingsBundle) []UserRatingsBundle {
	var (
		userName string
		userRatings string
	)
	for rows.Next() {
		err := rows.Scan(&userName, &userRatings)
		if err != nil {
			logToFile(err)
			return resultSet
		}
		defer rows.Close()
		if len(userRatings) <= 0 {
			userRatings = `{}`
		}
		res := map[string]int{}
		json.Unmarshal([]byte(userRatings), &res)
		resultSet = append(resultSet, UserRatingsBundle{userName, res})
	}
	err := rows.Err()
	if err != nil {
		logToFile(err)
		return resultSet
	}
	return resultSet
}

type GameRecommendation struct {
	Id int
	Name string
	Rating int
}

func recommend(userName string) []GameRecommendation {
	// Accessing a new regommend table for the first time will create it.
	games := regommend.Table("games")

	sampleRatings := fetchUserRatingsSample()
	for _, bundle := range sampleRatings {
		ratings := make(map[interface{}]float64)
		for gameId, gameRating := range bundle.ratings {
			if gameRating != 0 {
				ratings[gameId] = float64(gameRating)
			}
		}
		games.Add(bundle.userName, ratings)
	}

	recs, _ := games.Recommend(userName)
	recSize := int(math.Min(5,float64(len(recs))))
	recList := make(map[int]GameRecommendation)

	for i := 0; i < recSize ; i++ {
		if str, ok := recs[i].Key.(string); ok {
			id, _ := strconv.Atoi(str)
			recList[id] = GameRecommendation{Id: id, Rating: int(recs[i].Distance)}
		} else {
			/* not string */
		}
	}
	recList = getGameMetadataForIds(recList)
	vals := make([]GameRecommendation, 0)
	for _, val := range recList {
		vals = append(vals, val)
	}
	return vals
}

func getGameMetadataForIds(recMap map[int]GameRecommendation) map[int]GameRecommendation {
	var (
		id int
		gameName string
	)
	ret := make(map[int]GameRecommendation)
	keys := make([]interface{}, 0, len(recMap))
	for k := range recMap {
		keys = append(keys, k)
	}
	if (len(keys) < 1) {
		return recMap
	}
	stmt, err := sqlDb.Prepare("select id, gameName from game_metadata where id IN(?" + strings.Repeat(",?", len(keys)-1) + ")")
	if err != nil {
		logToFile(err)
		stmt.Close()
		return ret
	}
	defer stmt.Close()
	rows, err := stmt.Query(keys...)
	if err != nil {
		logToFile(err)
		rows.Close()
		return ret
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&id, &gameName)
		if err != nil {
			logToFile(err)
			continue
		}
		ret[id] = GameRecommendation{id, gameName, recMap[id].Rating}
	}
	err = rows.Err()
	if err != nil {
		logToFile(err)
		return ret
	}
	return ret
}

func logToFile(s ...interface{}) {
	log.SetOutput(os.Stdout)
	log.Println(s)
	currentDate := time.Now().UTC()
	dateString := fmt.Sprintf("%d-%02d-%02d", currentDate.Year(), currentDate.Month(), currentDate.Day())

	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", os.FileMode(0777))
	}
	f, err := os.OpenFile("logs/log-"+ operatingMode + "-" +dateString + ".txt", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println(s)
	log.SetOutput(os.Stdout)
}
