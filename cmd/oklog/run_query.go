package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/oklog/prototype/pkg/store"
)

func runQuery(args []string) error {
	flagset := flag.NewFlagSet("query", flag.ExitOnError)
	var (
		storeAddr = flagset.String("store", "localhost:7650", "okstore instance")
		from      = flagset.String("from", "-1h", "from, as RFC3339 timestamp or duration")
		to        = flagset.String("to", "now", "to, as RFC3339 timestamp or duration")
		q         = flagset.String("q", "", "query expression")
		engine    = flagset.String("engine", "lazy", "naïve, ripgrep, lazy")
		stats     = flagset.Bool("stats", false, "statistics only, no records")
		nocopy    = flagset.Bool("nocopy", false, "don't read the response body")
	)
	if err := flagset.Parse(args); err != nil {
		return err
	}

	begin := time.Now()

	fromDuration, durationErr := time.ParseDuration(*from)
	fromTime, timeErr := time.Parse(time.RFC3339Nano, *from)
	fromNow := strings.ToLower(*from) == "now"
	var fromStr string
	switch {
	case fromNow:
		fromStr = time.Now().Format(time.RFC3339Nano)
	case durationErr == nil && timeErr != nil:
		fromStr = time.Now().Add(fromDuration).Format(time.RFC3339Nano)
	case durationErr != nil && timeErr == nil:
		fromStr = fromTime.Format(time.RFC3339Nano)
	default:
		return fmt.Errorf("couldn't parse -from (%q) as either duration or time", *from)
	}

	toDuration, durationErr := time.ParseDuration(*to)
	toTime, timeErr := time.Parse(time.RFC3339Nano, *to)
	toNow := strings.ToLower(*to) == "now"
	var toStr string
	switch {
	case toNow:
		toStr = time.Now().Format(time.RFC3339Nano)
	case durationErr == nil && timeErr != nil:
		toStr = time.Now().Add(toDuration).Format(time.RFC3339Nano)
	case durationErr != nil && timeErr == nil:
		toStr = toTime.Format(time.RFC3339Nano)
	default:
		return fmt.Errorf("couldn't parse -to (%q) as either duration or time", *to)
	}

	fmt.Fprintf(os.Stderr, "-from %s -to %s\n", fromStr, toStr)

	method := "GET"
	if *stats {
		method = "HEAD"
	}

	// TODO(pb): use const or client lib for URL
	req, err := http.NewRequest(method, fmt.Sprintf(
		"http://%s/store/query?engine=%s&from=%s&to=%s&q=%s",
		*storeAddr,
		url.QueryEscape(*engine),
		url.QueryEscape(fromStr),
		url.QueryEscape(toStr),
		url.QueryEscape(*q),
	), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var result store.QueryResult
	result.DecodeFrom(resp)

	fmt.Fprintf(os.Stderr, "Response in %s\n", time.Since(begin))
	fmt.Fprintf(os.Stderr, "Used engine %s\n", result.Engine)
	fmt.Fprintf(os.Stderr, "Queried from %s\n", result.From)
	fmt.Fprintf(os.Stderr, "Queried to %s\n", result.To)
	fmt.Fprintf(os.Stderr, "Queried expression %q\n", result.Q)
	fmt.Fprintf(os.Stderr, "%d node(s) queried\n", result.NodesQueried)
	fmt.Fprintf(os.Stderr, "%d segment(s) queried\n", result.SegmentsQueried)
	fmt.Fprintf(os.Stderr, "%d record(s) queried\n", result.RecordsQueried)
	fmt.Fprintf(os.Stderr, "%d record(s) matched\n", result.RecordsMatched)
	fmt.Fprintf(os.Stderr, "%d error(s)\n", result.ErrorCount)

	if !*nocopy {
		io.Copy(os.Stdout, result.Records)
	}
	result.Records.Close()

	return nil
}
