package borgletlib

import (
	"flag"
)

var (
	byBorglet = flag.Bool("by_borglet", false, "is run by borglet")
)

func RunByBorglet() bool {
	return *byBorglet
}
