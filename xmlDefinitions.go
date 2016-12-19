package main

type UsersRated struct {
	Value int `xml:"value,attr"`
}
type AverageRating struct {
	Value float32 `xml:"value,attr"`
}
type BayesAverageRating struct {
	Value float32 `xml:"value,attr"`
}
type StdDevRating struct {
	Value float32 `xml:"value,attr"`
}
type MedianRating struct {
	Value int `xml:"value,attr"`
}
type Rating struct {
	Value int `xml:"value,attr"`
	UsersRated UsersRated `xml:"usersrated"`
	AverageRating AverageRating `xml:"average"`
	BayesAverageRating BayesAverageRating `xml:"bayesaverage"`
	StdDevRating StdDevRating `xml:"stddev"`
	MedianRating MedianRating `xml:"median"`
}
type Stats struct {
	MinPlayers int `xml:"minplayers,attr"`
	MaxPlayers int `xml:"maxplayers,attr"`
	MinPlaytime int `xml:"minplaytime,attr"`
	MaxPlaytime int `xml:"maxplaytime,attr"`
	PlayingTime int `xml:"playingtime,attr"`
	NumOwned int `xml:"numowned,attr"`
	Rating Rating `xml:"rating"`
}
type Item struct {
	Stats Stats `xml:"stats"`
}
type Items struct {
	Items []Item `xml:"item"`
}

//<item objecttype="thing" objectid="7865" subtype="boardgame" collid="1108162">
//<name sortindex="1">10 Days in Africa</name>
//<yearpublished>2003</yearpublished>
//<image>//cf.geekdo-images.com/images/pic1229634.jpg</image>
//<thumbnail>//cf.geekdo-images.com/images/pic1229634_t.jpg</thumbnail>
////<stats minplayers="2" maxplayers="4" minplaytime="25" maxplaytime="25" playingtime="25" numowned="1938">
////<rating value="8">
////<usersrated value="1709"/>
////<average value="6.56771"/>
////<bayesaverage value="6.18959"/>
////<stddev value="1.16497"/>
////<median value="0"/>
////</rating>
////</stats>
//<status own="1" prevowned="0" fortrade="0" want="0" wanttoplay="0" wanttobuy="0" wishlist="0" preordered="0" lastmodified="2007-10-12 22:51:58"/>
//<numplays>21</numplays>
//<comment>
//Light filler and as a nice side effect you learn some African geography.
//</comment>
//</item>
