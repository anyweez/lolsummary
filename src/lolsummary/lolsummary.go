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
	RATING_AVERAGE       = iota
	RATING_BELOW_AVERAGE = iota
	RATING_BOTTOM        = iota
)

const (
	DAY_RANGE = 3
)

type OutputRecord struct {
	Metrics  []Metric
	Summoner structs.ProcessedSummoner
}

type Metric struct {
	Name   string
	Values []MetricValue
}

type MetricValue struct {
	UserScore    float64
	LeagueMedian float64
	Rating       int
	RatingString string
	NumGames     int
}

var MONGO_CONNECTION_URL = flag.String("mongodb", "localhost", "The URL that mgo should use to connect to Mongo.")
var SUMMONER_FILE = flag.String("summoners", "input/pentakool", "Path to a file containing summoner ID's to evaluate.")
var CURRENT_SEASON_START = flag.String("season_start", "2015-01-21", "The start date of the current season (currently season 5).")

func GetRating(rating int) string {
	switch rating {
	case RATING_TOP:
		return "top of the league"
		break
	case RATING_ABOVE_AVERAGE:
		return "above average"
		break
	case RATING_BELOW_AVERAGE:
		return "below average"
		break
	case RATING_BOTTOM:
		return "bottom of the bucket"
		break
	case RATING_AVERAGE:
		return "average"
		break
	}

	// Should never happen
	return ""
}

/**
 * Fetch all recent games for the summoner provided. Note that games are pulled from processed logs
 * so anything that hasn't been processed yet isn't going to show up.
 */
func GetGamesForSummoner(summoner_id int, earliest_date string, latest_date string) ([]structs.ProcessedGame, error) {
	games := make([]structs.ProcessedGame, 0)

	// Create the Mongo session.
	session, cerr := mgo.Dial(*MONGO_CONNECTION_URL)
	if cerr != nil {
		return games, cerr
	}
	collection := session.DB("league").C("processed_games")

	// TODO: add "where game is recent" filter
	collection.Find(bson.M{
		"stats.summonerid": summoner_id,
		"gamedate": bson.M{
			"$gt": earliest_date,
			"$lt": latest_date,
		},
	}).All(&games)

	// TODO: drop stats for all players except for the requested player.
	return games, nil
}

func GetGamesForLeague(tier string, division int, earliest_date string, latest_date string) ([]structs.ProcessedGame, error) {
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
		"gamedate": bson.M{
			"$gt": earliest_date,
			"$lt": latest_date,
		},
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

// func GetMetricsForDateRange(summoner structs.ProcessedSummoner, start_date time.Time, end_date time.Time) []Metric {
// 	// Retrieve the appropriate games.
// 	summoner_games, _ := GetGamesForSummoner(summoner.SummonerId, start_date.Format("2006-01-02"), end_date.Format("2006-01-02"))
// 	league_games, _ := GetGamesForLeague(summoner.CurrentTier, summoner.CurrentDivision, start_date.Format("2006-01-02"), end_date.Format("2006-01-02"))

// 	metrics := make([]Metric, 0)

// 	////
// 	// Compute stats
// 	////

// 	// Stats for CS
// 	cs := buildMetric(summoner_games, league_games, "Creep score", func(game structs.ProcessedGame) float64 {
// 		return float64(game.Stats[0].MinionsKilled)
// 	}, max)
// 	metrics = append(metrics, cs)

// 	// Stats for # of deaths
// 	deaths := buildMetric(summoner_games, league_games, "Deaths", func(game structs.ProcessedGame) float64 {
// 		return float64(game.Stats[0].NumDeaths)
// 	}, min)
// 	metrics = append(metrics, deaths)

// 	// Stats for # of wards placed
// 	wardsPlaced := buildMetric(summoner_games, league_games, "Wards placed", func(game structs.ProcessedGame) float64 {
// 		return float64(game.Stats[0].WardsPlaced)
// 	}, max)
// 	metrics = append(metrics, wardsPlaced)

// 	// Stats for # of wards cleared
// 	wardsCleared := buildMetric(summoner_games, league_games, "Wards cleared", func(game structs.ProcessedGame) float64 {
// 		return float64(game.Stats[0].WardsCleared)
// 	}, max)
// 	metrics = append(metrics, wardsCleared)

// 	return metrics
// }

func GetMetricsForDateRanges(summoner structs.ProcessedSummoner, start_dates []time.Time, end_date time.Time) []Metric {
	metrics := make([]Metric, 0)

	for i, start := range start_dates {
		summoner_games, _ := GetGamesForSummoner(summoner.SummonerId, start.Format("2006-01-02"), end_date.Format("2006-01-02"))
		league_games, _ := GetGamesForLeague(summoner.CurrentTier, summoner.CurrentDivision, start.Format("2006-01-02"), end_date.Format("2006-01-02"))

		fmt.Println(fmt.Sprintf("  [start=%s] summoner games: %d, league games: %d", start, len(summoner_games), len(league_games)))

		// Create an object for each metric the first time, and populate it with the data for the first start_date.
		if i == 0 {
			// Creep score
			metrics = append(metrics, buildMetric(summoner_games, league_games, "Creep score", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].MinionsKilled)
			}, max))

			// Deaths
			metrics = append(metrics, buildMetric(summoner_games, league_games, "Deaths", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].NumDeaths)
			}, min))

			metrics = append(metrics, buildMetric(summoner_games, league_games, "Wards placed", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].WardsPlaced)
			}, max))

			metrics = append(metrics, buildMetric(summoner_games, league_games, "Wards cleared", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].WardsCleared)
			}, max))
			// Everything after the first should be appended to the first.
		} else {
			// TODO: Add other metrics.
			cs := buildMetric(summoner_games, league_games, "Creep score", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].MinionsKilled)
			}, max)
			metrics[0].Values = append(metrics[0].Values, cs.Values...)

			cs = buildMetric(summoner_games, league_games, "Deaths", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].NumDeaths)
			}, max)
			metrics[1].Values = append(metrics[1].Values, cs.Values...)

			cs = buildMetric(summoner_games, league_games, "Wards placed", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].WardsPlaced)
			}, max)
			metrics[2].Values = append(metrics[2].Values, cs.Values...)

			cs = buildMetric(summoner_games, league_games, "Wards cleared", func(game structs.ProcessedGame) float64 {
				return float64(game.Stats[0].WardsCleared)
			}, max)
			metrics[3].Values = append(metrics[3].Values, cs.Values...)
		}
	}

	return metrics
}

func main() {
	flag.Parse()
	// TODO: read summoner ID's in from somewhere.
	log.Println("Loading summoner id's from " + *SUMMONER_FILE)
	summoner_ids, err := shared.LoadIds(*SUMMONER_FILE)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println(fmt.Sprintf("Done; loaded %d summoner id's", len(summoner_ids)))

	output := make(map[int]OutputRecord)

	// For each summoner, compute a bunch of stats.
	for _, summoner_id := range summoner_ids {
		summoner, err := GetProcessedSummoner(summoner_id)
		if err != nil {
			log.Fatal(err.Error())
		}

		// TODO: this should probably be a constant somewhere.
		season_start, err := time.Parse("2006-01-02", *CURRENT_SEASON_START)
		if err != nil {
			fmt.Println("Couldn't parse season start timestamp.")
			return
		}

		fmt.Println(fmt.Sprintf("Summoner #%d", summoner.SummonerId))
		// Add the summoner and instantiate the list of metrics for this summoner.
		output[summoner_id] = OutputRecord{
			Summoner: summoner,
			Metrics: GetMetricsForDateRanges(summoner, []time.Time{
				// Three days ago.
				time.Unix(time.Now().Unix()-int64(3*86400), 0),
				// Seven days ago.
				time.Unix(time.Now().Unix()-int64(7*86400), 0),
				// Thirty days ago.
				time.Unix(time.Now().Unix()-int64(30*86400), 0),
				// Start of season five.
				season_start,
			}, time.Now()),
		}
	}

	WriteMetrics(output, "pentakool.html")
}

/**
 * Generate a Metric object from the provided games.
 */
func buildMetric(playerGames []structs.ProcessedGame, leagueGames []structs.ProcessedGame, name string, sampleFunc func(game structs.ProcessedGame) float64, ratingFunc func(float64, float64, float64) int) Metric {
	values := MetricValue{}

	sample := make([]float64, 0)
	for _, game := range playerGames {
		sample = append(sample, sampleFunc(game))
	}
	values.UserScore = stats.Median(sample)

	sample = make([]float64, 0)
	for _, game := range leagueGames {
		sample = append(sample, sampleFunc(game))
	}

	values.LeagueMedian = stats.Median(sample)
	values.Rating = ratingFunc(values.UserScore, values.LeagueMedian, stats.StdDevS(sample))
	values.RatingString = GetRating(values.Rating)
	values.NumGames = len(playerGames)

	return Metric{
		Name:   name,
		Values: []MetricValue{values},
	}
}
