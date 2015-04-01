package main

func max(score float64, median float64, stddev float64) int {
	switch {
	case score < (median - stddev):
		return RATING_BOTTOM
	case score < median:
		return RATING_BELOW_AVERAGE
	case score > (median + stddev):
		return RATING_TOP
	case score > median:
		return RATING_ABOVE_AVERAGE
	}

	return -1
}

func min(score float64, median float64, stddev float64) int {
	switch {
	case score < (median - stddev):
		return RATING_TOP
	case score < median:
		return RATING_ABOVE_AVERAGE
	case score > (median + stddev):
		return RATING_BOTTOM
	case score > median:
		return RATING_BELOW_AVERAGE
	}

	return -1
}
