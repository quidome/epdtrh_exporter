package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	log           = logrus.WithField("pkg", "main")
	debug         = kingpin.Flag("debug", "enable debugging").Short('d').Bool()
	listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":2112").String()
)

func readThermo(target string) (therm, humidity, dewPoint float32, err error) {
	query := fmt.Sprintf("http://%s/hodnen.html", target)

	log.Debugf("get data from %s", query)

	resp, err := http.Get(query)
	if err != nil {
		// handle error
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		// handle error
		return 0, 0, 0, err
	}

	fmt.Print(body)
	return 23.1, 50.5, 12.4, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["target"]

	if !ok || len(keys[0]) < 1 {
		log.Println("Url Param 'key' is missing")
		return
	}

	// Only use first target
	target := keys[0]

	// get data from target
	temperature, humidity, dewPoint, err := readThermo(target)

	if err != nil {
		// handle error
		log.Error(err)
	} else {
		fmt.Printf("T: %f\nH: %f\nD: %f\n", temperature, humidity, dewPoint)
	}
}

func main() {
	kingpin.Parse()

	// configure logging
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true})
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	log.Infof("Starting listener on %s", *listenAddress)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/therm", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
	http.ListenAndServe(*listenAddress, nil)
}
