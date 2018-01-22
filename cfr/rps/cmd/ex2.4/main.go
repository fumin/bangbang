package main

import (
	"flag"

	"github.com/fumin/bangbang/cfr/rps"
	"github.com/golang/glog"
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	cfr := rps.NewRPS()
	oppStrategy := []float64{0.4, 0.3, 0.3}
	rps.Train(cfr, oppStrategy, 1000000)
	glog.Infof("average strategy: %+v", cfr.GetAverageStrategy())
}
