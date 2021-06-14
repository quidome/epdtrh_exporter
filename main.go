package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	log           = logrus.WithField("pkg", "main")
	debug         = kingpin.Flag("debug", "enable debugging").Short('d').Bool()
	listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":2112").String()
)

func floatFromString(measurement string) (float32, error) {
	// strip float from beginning of string
	r, _ := regexp.Compile("^[0-9]*.?[0-9]*")
	strippedValue := r.FindString(measurement)
	log.Debug(strippedValue)
	float, err := strconv.ParseFloat(strippedValue, 32)
	if err != nil {
		return 0, err
	}

	return float32(float), nil
}

func pullDataFromHTML(rawData []uint8) (temperature, humidity, dewPoint float32, err error) {
	var floatjes []float32
	document := string(rawData)

	// fmt.Print(string(rawData))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(document))
	if err != nil {
		fmt.Println("No url found")
		log.Error(err)
		return 0, 0, 0, err
	}

	doc.Find("table").EachWithBreak(func(index int, tablehtml *goquery.Selection) bool {
		log.Debugf("index: %d", index)
		tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
			log.Debugf("indextr: %d", indextr)
			// rowhtml.Find("th").Each(func(indexth int, tableheading *goquery.Selection) {
			// 	log.Debugf("indexth: %d", indexth)
			// 	headings = append(headings, tableheading.Text())
			// })
			rowhtml.Find("td").Each(func(indextd int, tablecell *goquery.Selection) {
				// skip indextd 0, contains the name of the value
				if indextd == 1 {
					floatValue, err := floatFromString(tablecell.Text())
					if err != nil {
						log.Error("cannot convert ", tablecell.Text())
					}
					floatjes = append(floatjes, floatValue)
					log.Debugf("indextd: %d\tvalue: %f", indextd, floatValue)
				}
			})
		})
		return false
	})
	return floatjes[0], floatjes[1], floatjes[2], nil
}

func readThermo(target string) (therm, humidity, dewPoint float32, err error) {
	host := fmt.Sprintf("%s:80", target)
	log.Debugf("get data from %s", target)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Error(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET /hodnen.html HTTP/1.0\r\n\r\n")
	msg, err := ioutil.ReadAll(conn)

	if err != nil {
		logrus.Error(err)
		return 0, 0, 0, err
	}

	t, h, d, err := pullDataFromHTML(msg)
	if err != nil {
		return 0, 0, 0, err
	}

	return t, h, d, nil
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
		log.Debug("Log level switched to debug")
	}

	log.Infof("Starting listener on %s", *listenAddress)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/therm", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
	http.ListenAndServe(*listenAddress, nil)
}
