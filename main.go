package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	log           = logrus.WithField("pkg", "main")
	debug         = kingpin.Flag("debug", "enable debugging").Short('d').Bool()
	listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":2112").String()

	promTemperature = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "epdtrh",
			Name:      "temperature",
			Help:      "shows temperature from epdtrh",
		},
		[]string{"instance"},
	)

	promHumidity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "epdtrh",
			Name:      "humidity",
			Help:      "shows humidity from epdtrh",
		},
		[]string{"instance"},
	)

	promDewPoint = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "epdtrh",
			Name:      "dewpoint",
			Help:      "shows dewpoint from epdtrh",
		},
		[]string{"instance"})
)

func init() {
	prometheus.Register(promTemperature)
	prometheus.Register(promHumidity)
	prometheus.Register(promDewPoint)
}

func floatFromString(measurement string) (float32, error) {
	// strip float from string
	r, _ := regexp.Compile(`\d*\.?\d*`)
	strippedValue := r.FindString(measurement)
	log.Debug(strippedValue)
	float, err := strconv.ParseFloat(strippedValue, 32)
	if err != nil {
		return 0, err
	}

	return float32(float), nil
}

func pullDataFromHTML(rawData []uint8) (temperature, humidity, dewPoint float32, err error) {
	var measurements []float32
	document := string(rawData)

	// fmt.Print(string(rawData))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(document))
	if err != nil {
		log.Error("No url found")
		log.Error(err)
		return 0, 0, 0, err
	}

	// crawl through the document with the intention to find 3 measurements
	doc.Find("table").EachWithBreak(func(index int, tablehtml *goquery.Selection) bool {
		log.Debugf("index: %d", index)
		tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
			log.Debugf("indextr: %d", indextr)
			rowhtml.Find("td").Each(func(indextd int, tablecell *goquery.Selection) {
				// skip indextd 0, contains the name of the value
				if indextd == 1 {
					floatValue, err := floatFromString(tablecell.Text())
					if err != nil {
						log.Error("cannot convert ", tablecell.Text())
					}
					measurements = append(measurements, floatValue)
					log.Debugf("indextd: %d\tvalue: %f", indextd, floatValue)
				}
			})
		})
		return false
	})
	return measurements[0], measurements[1], measurements[2], nil
}

func readThermo(target string) (temperature, humidity, dewpoint float32, err error) {
	document := "/hodnen.html"
	host := fmt.Sprintf("%s:80", target)
	log.Debugf("get %s from %s", document, host)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Error(err)
		return 0, 0, 0, fmt.Errorf("cannot connect to %s", host)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "GET "+document+" HTTP/1.0\r\n\r\n")
	response, err := io.ReadAll(conn)

	if err != nil {
		logrus.Error(err)
		return 0, 0, 0, fmt.Errorf("could not read %s", document)
	}

	temperature, humidity, dewpoint, err = pullDataFromHTML(response)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not read metrics from data")
	}
	return
}

func handler(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["target"]

	if !ok || len(keys[0]) < 1 {
		log.Println("Url parameter 'target' is missing")
		return
	}

	// Only use first target
	target := keys[0]

	// get data from target
	temperature, humidity, dewPoint, err := readThermo(target)
	log.Printf("T: %f\tH: %f\tD: %f\t", temperature, humidity, dewPoint)

	if err != nil {
		// handle error
		log.Error(err)
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
	} else {
		// add data to exporter
		promTemperature.WithLabelValues(target).Set(float64(temperature))
		promHumidity.WithLabelValues(target).Set(float64(humidity))
		promDewPoint.WithLabelValues(target).Set(float64(dewPoint))

		// write exporter
		promhttp.Handler().ServeHTTP(w, r)

		promTemperature.Reset()
		promHumidity.Reset()
		promDewPoint.Reset()

	}
}

func main() {
	kingpin.Parse()

	// configure logging
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true})
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
		log.Debug("Log level switched to debug")
	}

	log.Infof("Starting listener on %s", *listenAddress)
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})

	err := http.ListenAndServe(*listenAddress, nil)

	if err != nil {
		log.Error("cannot start server: " + err.Error())
	}
}
