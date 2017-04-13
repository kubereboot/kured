package main

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/weaveworks/kured/pkg/alerts"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("USAGE: %s <prometheusURL> <filterRegexp>", os.Args[0])
	}

	count, err := alerts.PrometheusCountActive(os.Args[1], regexp.MustCompile(os.Args[2]))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(count)
}
