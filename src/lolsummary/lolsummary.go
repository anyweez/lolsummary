package main

// TODO: processed_summoners odn't appear to have their rank. Confirm that it's actually in raw.

import (
	"flag"
	"fmt"
	shared "github.com/luke-segars/loldata/src/shared"
	"github.com/luke-segars/loldata/src/shared/structs"
	"github.com/montanaflynn/stats"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"time"
)

const (
	RATING_TOP           = iota
	RATING_ABOVE_AVERAGE = iota
	RATING_BELOW_AVERAGE = iota
	RATING_BOTTOM        = iota
)

const (
	DAY_RANGE = 3
)

type Metric struct {
	Name         string
	UserScore    float64
	LeagueMedian float64
	Rating       int
}

var MONGO_CONNECTION_URL = flag.String("mongodb", "localhost", "The URL that mgo should use to connect to Mongo.")
var SUMMONER_FILE = flag.String("summoners", "input/pentakool", "Path to a file containing summoner ID's to evaluate.")

/**
 * Fetch all recent games for the summoner provided. Note that games are pulled from processed logs
 * so anything that hasn't been processed yet isn't going to show up.
 */
func GetGamesForSummoner(summoner_id int, earliest_date string) ([]structs.ProcessedGame, error) {
	games := make([]structs.ProcessedGame, 0)
	// Create the Mongo session.
	session, cerr := mgo.Dial(*MONGO_CONNECTION_URL)
	if cerr != nil {
		return games, cerr
	}
	collection := session.DB("league").C("processed_games")

	// TODO: add "where game is recent" filter
	collection.Find(bson.M{"stats.summonerid": summoner_id}).All(&games)

	// TODO: drop stats for all players except for the requested player.
	return games, nil
}

func GetGamesForLeague(tier string, division int, earliest_date string) ([]structs.ProcessedGame, error) {
	games := make([]structs.ProcessedGame, 0)

	// Create the Mongo session.
	session, cerr := mgo.Dial(*MONGO_CONNECTION_URL)
	if cerr != nil {
		return games, cerr
	}
	collection := session.DB("league").C("processed_games")

	// TODO: add "where game is recent" filter
	collection.Find(bson.M{
		"stats.summonertier":     tier,
		"stats.summonerdivision": division,
	}).All(&games)

	return games, nil
}

func GetProcessedSummoner(summoner_id int) (structs.ProcessedSummoner, error) {
	summoner := structs.ProcessedSummoner{}
	// Create the Mongo session.
	session, cerr := mgo.Dial(*MONGO_CONNECTION_URL)
	if cerr != nil {
		return summoner, cerr
	}
	collection := session.DB("league").C("processed_summoners")

	// TODO: add "where game is recent" filter
	collection.Find(bson.M{
		"_id": summoner_id,
	}).One(&summoner)

	return summoner, nil
}

func main() {
	flag.Parse()
	// TODO: read summoner ID's in from somewhere.
	summoner_ids, err := shared.LoadIds(*SUMMONER_FILE)
	if err != nil {
		log.Fatal(err.Error())
	}

	metrics := make(map[int][]Metric)

	// For each summoner, compute a bunch of stats.
	for _, summoner_id := range summoner_ids {
		summoner, err := GetProcessedSummoner(summoner_id)
		if err != nil {
			log.Fatal(err.Error())
		}

		// Instantiate the list of metrics for this summoner.
		metrics[summoner_id] = make([]Metric, 0)

		// Get a time representing "DAY_RANGE days ago"
		start := time.Unix(time.Now().Unix()-int64(DAY_RANGE*86400), 0)
		// Retrieve the appropriate games.
		summoner_games, _ := GetGamesForSummoner(summoner_id, start.Format("2006-01-02"))
		league_games, _ := GetGamesForLeague(summoner.CurrentTier, summoner.CurrentDivision, start.Format("2006-01-02"))

		////
		// Compute stats
		////

		// Stats for CS
		cs := buildMetric(summoner_games, league_games, "Creep score", func(game structs.ProcessedGame) float64 {
			return float64(game.Stats[0].MinionsKilled)
		})
		metrics[summoner_id] = append(metrics[summoner_id], cs)

		// Stats for # of deaths
		deaths := buildMetric(summoner_games, league_games, "Deaths", func(game structs.ProcessedGame) float64 {
			return float64(game.Stats[0].NumDeaths)
		})
		metrics[summoner_id] = append(metrics[summoner_id], deaths)

		// Stats for # of wards placed
		wardsPlaced := buildMetric(summoner_games, league_games, "Wards placed", func(game structs.ProcessedGame) float64 {
			return float64(game.Stats[0].WardsPlaced)
		})
		metrics[summoner_id] = append(metrics[summoner_id], wardsPlaced)

		// Stats for # of wards cleared
		wardsCleared := buildMetric(summoner_games, league_games, "Wards cleared", func(game structs.ProcessedGame) float64 {
			return float64(game.Stats[0].WardsCleared)
		})
		metrics[summoner_id] = append(metrics[summoner_id], wardsCleared)

		// Print a report for each summoner.
		fmt.Println(summoner.SummonerId)
		for _, metric := range metrics[summoner_id] {
			fmt.Println(fmt.Sprintf("  - %s: %.2f vs %.2f (%d)", metric.Name, metric.UserScore, metric.LeagueMedian, metric.Rating))
		}
	}
}

/**
 * Generate a Metric object from the provided games.
 */
func buildMetric(playerGames []structs.ProcessedGame, leagueGames []structs.ProcessedGame, name string, sampleFunc func(game structs.ProcessedGame) float64) Metric {
	metric := Metric{
		Name: name,
	}

	sample := make([]float64, 0)
	for _, game := range playerGames {
		sample = append(sample, sampleFunc(game))
	}
	metric.UserScore = stats.Median(sample)

	sample = make([]float64, 0)
	for _, game := range leagueGames {
		sample = append(sample, sampleFunc(game))
	}
	metric.LeagueMedian = stats.Median(sample)

	//	variance := stats.Variance(sample, len(league_games))
	stddev := stats.StdDevS(sample)

	switch {
	case metric.UserScore < (metric.LeagueMedian - stddev):
		metric.Rating = RATING_BOTTOM
		break
	case metric.UserScore < metric.LeagueMedian:
		metric.Rating = RATING_BELOW_AVERAGE
		break
	case metric.UserScore > (metric.LeagueMedian + stddev):
		metric.Rating = RATING_TOP
		break
	case metric.UserScore > metric.LeagueMedian:
		metric.Rating = RATING_ABOVE_AVERAGE
		break
	}

	return metric
}
