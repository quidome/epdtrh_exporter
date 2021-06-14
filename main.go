package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	log   = logrus.WithField("pkg", "main")
	debug = kingpin.Flag("debug", "enable debugging").Short('d').Bool()
)

func recordMetrics() {
	go func() {
		for {
			opsProcessed.Inc()
			time.Sleep(2 * time.Second)
		}
	}()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
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

	log.Info("Starting listener")
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/therm", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
	http.ListenAndServe(":2112", nil)
}
