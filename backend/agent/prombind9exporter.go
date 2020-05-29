package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	storkutil "isc.org/stork/util"
)

// Settings for Prometheus BIND 9 Exporter
type PromBind9ExporterSettings struct {
	Host     string `long:"prometheus-bind9-exporter-host" description:"the IP to listen on" default:"0.0.0.0" env:"STORK_AGENT_PROMETHEUS_BIND9_EXPORTER_ADDRESS"`
	Port     int    `long:"prometheus-bind9-exporter-port" description:"the port to listen on for connections" default:"9548" env:"STORK_AGENT_PROMETHEUS_BIND9_EXPORTER_PORT"`
	Interval int    `long:"prometheus-bind9-exporter-interval" description:"interval of collecting BIND 9 stats in seconds" default:"10" env:"STORK_AGENT_PROMETHEUS_BIND9_EXPORTER_INTERVAL"`
}

const (
	namespace = "bind9"
)

type PromBind9ViewStats struct {
	ResolverCachestats map[string]float64
}

// Statistics to be exported.
type PromBind9ExporterStats struct {
	BootTime        time.Time
	ConfigTime      time.Time
	CurrentTime     time.Time
	IncomingQueries map[string]float64
	Views           map[string]PromBind9ViewStats
}

// Main structure for Prometheus BIND 9 Exporter. It holds its settings,
// references to app monitor, HTTP client, HTTP server, and main loop
// controlling elements like ticker, and mappings between BIND 9 stats
// names to prometheus stats.
type PromBind9Exporter struct {
	Settings PromBind9ExporterSettings

	AppMonitor AppMonitor
	HTTPClient *HTTPClient
	HTTPServer *http.Server

	Ticker        *time.Ticker
	DoneCollector chan bool
	Wg            *sync.WaitGroup

	up              prometheus.Gauge
	serverStatsDesc map[string]*prometheus.Desc
	viewStatsDesc   map[string]*prometheus.Desc

	stats PromBind9ExporterStats
}

// Create new Prometheus BIND 9 Exporter.
func NewPromBind9Exporter(appMonitor AppMonitor) *PromBind9Exporter {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Handler: mux,
	}
	pbe := &PromBind9Exporter{
		AppMonitor:    appMonitor,
		HTTPClient:    NewHTTPClient(),
		HTTPServer:    srv,
		DoneCollector: make(chan bool),
		Wg:            &sync.WaitGroup{},
	}

	pbe.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "up",
		Help:      "Was the Bind instance query successful?",
	})

	// bind_exporter stats
	serverStatsDesc := make(map[string]*prometheus.Desc)
	viewStatsDesc := make(map[string]*prometheus.Desc)

	// boot_time_seconds
	serverStatsDesc["boot-time"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "boot_time_seconds"),
		"Start time of the BIND process since unix epoch in seconds.", nil, nil)
	// config_time_seconds
	serverStatsDesc["config-time"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "config_time_seconds"),
		"Time of the last reconfiguration since unix epoch in seconds.", nil, nil)
	// current_time_seconds
	serverStatsDesc["current-time"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "current_time_seconds"),
		"Current time unix epoch in seconds as reported by named.", nil, nil)

	// exporter_build_info

	// incoming_queries_total
	serverStatsDesc["qtypes"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "incoming_queries_total"),
		"Number of incoming DNS queries.", []string{"type"}, nil)

	// resolver_cache_hit_ratio
	viewStatsDesc["CacheHitRatio"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "cache_hit_ratio"),
		"Cache effectiveness (cache hit ratio).", []string{"view"}, nil)
	// resolver_cache_hits
	viewStatsDesc["CacheHits"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "cache_hits"),
		"Total number of cache hits.", []string{"view"}, nil)
	// resolver_cache_misses
	viewStatsDesc["CacheMisses"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "cache_misses"),
		"Total number of cache misses.", []string{"view"}, nil)
	// resolver_query_hit_ratio
	viewStatsDesc["QueryHitRatio"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "query_hit_ratio"),
		"Query effectiveness (query hit ratio).", []string{"view"}, nil)
	// resolver_query_hits
	viewStatsDesc["QueryHits"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "query_hits"),
		"Total number of queries that were answered from cache.", []string{"view"}, nil)
	// resolver_query_misses
	viewStatsDesc["QueryMisses"] = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "resolver", "query_misses"),
		"Total number of queries that were not in cache.", []string{"view"}, nil)

	// incoming_requests_total
	// process_cpu_seconds_total
	// process_max_fds
	// process_open_fds
	// process_resident_memory_bytes
	// process_start_time_seconds
	// process_virtural_memory_bytes
	// process_virtural_memory_max_bytes
	// query_duplicates_total
	// query_errors_total
	// query_recursions_total
	// resolver_cache_rrsets
	// resolver_dnssec_validation_errors_total
	// resolver_dnssec_validation_success_total
	// resolver_queries_total
	// resolver_query_duration_seconds_bucket
	// resolver_query_duration_seconds_count
	// resolver_query_duration_seconds_sum
	// resolver_query_edns0_errors_total
	// resolver_query_errors_total
	// resolver_query_retries_total
	// resolver_response_errors_total
	// resolver_response_lame_total
	// resolver_response_mismatch_total
	// resolver_response_truncated_total
	// resolver_response_mismatch_total
	// response_total
	// tasks_running
	// up
	// worker_threads
	// zone_transfer_failure_total
	// zone_transfer_rejected_total
	// zone_transfer_success_total

	pbe.serverStatsDesc = serverStatsDesc
	pbe.viewStatsDesc = viewStatsDesc

	incomingQueries := make(map[string]float64)
	views := make(map[string]PromBind9ViewStats)
	pbe.stats = PromBind9ExporterStats{
		IncomingQueries: incomingQueries,
		Views:           views,
	}
	return pbe
}

// Describe describes all exported metrics. It implements prometheus.Collector.
func (pbe *PromBind9Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range pbe.serverStatsDesc {
		ch <- m
	}
	for _, m := range pbe.viewStatsDesc {
		ch <- m
	}
}

// collectTime fetches time stats.
func (pbe *PromBind9Exporter) collectTime(ch chan<- prometheus.Metric, key string, timeStat time.Time) {
	if !timeStat.IsZero() {
		ch <- prometheus.MustNewConstMetric(
			pbe.serverStatsDesc[key],
			prometheus.GaugeValue,
			float64(timeStat.Unix()))
	}
}

// Collect fetches the stats from configured location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (pbe *PromBind9Exporter) Collect(ch chan<- prometheus.Metric) {
	err := pbe.collectStats()
	if err != nil {
		log.Errorf("some errors were encountered while collecting stats from BIND 9: %+v", err)
		return
	}

	// boot_time_seconds
	pbe.collectTime(ch, "boot-time", pbe.stats.BootTime)
	// config_time_seconds
	pbe.collectTime(ch, "config-time", pbe.stats.ConfigTime)
	// current_time_seconds
	pbe.collectTime(ch, "current-time", pbe.stats.CurrentTime)

	// incoming_queries_total
	for label, value := range pbe.stats.IncomingQueries {
		ch <- prometheus.MustNewConstMetric(
			pbe.serverStatsDesc["qtypes"],
			prometheus.CounterValue,
			value, label)
	}

	// resolver_cache_hit_ratio
	// resolver_cache_hits
	// resolver_cache_misses
	// resolver_query_hit_ratio
	// resolver_query_hits
	// resolver_query_misses
	for view, viewStats := range pbe.stats.Views {
		for statName, statValue := range viewStats.ResolverCachestats {
			if desc, ok := pbe.viewStatsDesc[statName]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc, prometheus.CounterValue,
					statValue, view)
			}
		}
	}

}

// Start goroutine with main loop for collecting stats and HTTP server for
// exposing them to Prometheus.
func (pbe *PromBind9Exporter) Start() {
	// initial collect
	err := pbe.collectStats()
	if err != nil {
		log.Errorf("some errors were encountered while collecting stats from BIND 9: %+v", err)
		return
	}

	// register collectors
	prometheus.MustRegister(pbe)

	// set address for listening from settings
	addrPort := fmt.Sprintf("%s:%d", pbe.Settings.Host, pbe.Settings.Port)
	pbe.HTTPServer.Addr = addrPort

	log.Printf("Prometheus BIND 9 Exporter listening on %s, stats pulling interval: %d seconds", addrPort, pbe.Settings.Interval)

	// start HTTP server for metrics
	go func() {
		err := pbe.HTTPServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorf("problem with serving Prometheus BIND 9 Exporter: %s", err.Error())
		}
	}()
}

// Shutdown exporter goroutines and unregister prometheus stats.
func (pbe *PromBind9Exporter) Shutdown() {
	log.Printf("Stopping Prometheus BIND 9 Exporter")

	// stop http server
	if pbe.HTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pbe.HTTPServer.SetKeepAlivesEnabled(false)
		if err := pbe.HTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Could not gracefully shutdown the BIND 9 exporter: %v\n", err)
		}
	}

	// unregister bind9 counters from prometheus framework
	prometheus.Unregister(pbe)

	log.Printf("Stopped Prometheus BIND 9 Exporter")
}

// setDaemonStats stores the stat values from a daemon in the proper prometheus object.
func (pbe *PromBind9Exporter) setDaemonStats(rspIfc interface{}) error {
	rsp, ok := rspIfc.(map[string]interface{})
	if !ok {
		return errors.Errorf("problem with casting rspIfc: %+v", rspIfc)
	}

	var timeVal time.Time
	var timeStr string
	var err error
	getStat := func(statName string) interface{} {
		value, ok := rsp[statName]
		if !ok {
			log.Errorf("no '%s' in response:", statName)
			return nil
		}
		return value

	}
	// boot_time_seconds
	timeStr = getStat("boot-time").(string)
	timeVal, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return errors.Errorf("problem with parsing time %+s: %+v", timeStr, err)
	}
	pbe.stats.BootTime = timeVal

	// config_time_seconds
	timeStr = getStat("config-time").(string)
	timeVal, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return errors.Errorf("problem with parsing time %+s: %+v", timeStr, err)
	}
	pbe.stats.ConfigTime = timeVal

	// current_time_seconds
	timeStr = getStat("current-time").(string)
	timeVal, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return errors.Errorf("problem with parsing time %+s: %+v", timeStr, err)
	}
	pbe.stats.CurrentTime = timeVal

	// incoming_queries_total
	qtypesIfc := getStat("qtypes")
	if qtypesIfc != nil {
		qtypes, ok := qtypesIfc.(map[string]interface{})
		if !ok {
			return errors.Errorf("problem with casting 'qtypes' interface")
		}
		for labelName, labelValueIfc := range qtypes {
			// get value
			labelValue, ok := labelValueIfc.(float64)
			if !ok {
				log.Errorf("problem with casting %s labelValue: %+v", labelName, labelValueIfc)
				continue
			}
			// store stat value
			pbe.stats.IncomingQueries[labelName] = labelValue
		}
	}

	// Parse views.
	viewsIfc, ok := rsp["views"]
	if !ok {
		return errors.Errorf("no 'views' in response: %+v", rsp)
	}

	views := viewsIfc.(map[string]interface{})
	if !ok {
		return errors.Errorf("problem with casting viewsIfc: %+v", viewsIfc)
	}

	for viewName, viewStatsIfc := range views {
		pbe.initViewStats(viewName)

		viewStats, ok := viewStatsIfc.(map[string]interface{})
		if !ok {
			log.Errorf("problem with casting viewStatsIfc: %+v", viewStatsIfc)
			continue
		}
		// Parse resolver.
		resolverIfc, ok := viewStats["resolver"]
		if !ok {
			log.Errorf("no 'resolver' in viewStats: %+v", viewStats)
			continue
		}
		resolver, ok := resolverIfc.(map[string]interface{})
		if !ok {
			log.Errorf("problem with casting resolverIfc: %+v", resolverIfc)
			continue
		}

		// Parse cachestats.
		cachestatsIfc, ok := resolver["cachestats"]
		if !ok {
			log.Errorf("no 'cachestats' in resolver: %+v", resolver)
			continue
		}
		cachestats, ok := cachestatsIfc.(map[string]interface{})
		if !ok {
			log.Errorf("problem with casting cachestatsIfc: %+v", cachestatsIfc)
			continue
		}

		// resolver_cache_hit_ratio
		// resolver_cache_hits
		// resolver_cache_misses
		// resolver_query_hit_ratio
		// resolver_query_hits
		// resolver_query_misses
		var cacheHits float64
		var cacheMisses float64
		var queryHits float64
		var queryMisses float64
		for statName, statValueIfc := range cachestats {
			// get stat value
			statValue, ok := statValueIfc.(float64)
			if !ok {
				log.Errorf("problem with casting statValue: %+v", statValueIfc)
				continue
			}
			switch statName {
			case "CacheHits":
				cacheHits = statValue
			case "CacheMisses":
				cacheMisses = statValue
			case "QueryHits":
				queryHits = statValue
			case "QueryMisses":
				queryMisses = statValue
			}

			// store stat value
			pbe.stats.Views[viewName].ResolverCachestats[statName] = statValue
		}
		total := cacheHits + cacheMisses
		if total > 0 {
			pbe.stats.Views[viewName].ResolverCachestats["CacheHitRatio"] = cacheHits / total
		}
		total = queryHits + queryMisses
		if total > 0 {
			pbe.stats.Views[viewName].ResolverCachestats["QueryHitRatio"] = queryHits / total
		}
	}

	return nil
}

// collecStats collects stats from all bind9 apps.
func (pbe *PromBind9Exporter) collectStats() error {
	var lastErr error

	// Request to named statistics-channel for getting all server stats.
	request := `{}`

	// go through all bind9 apps discovered by monitor and query them for stats
	apps := pbe.AppMonitor.GetApps()
	for _, app := range apps {
		// ignore non-bind9 apps
		if app.Type != AppTypeBind9 {
			continue
		}

		// get stats from named
		sap, err := getAccessPoint(app, AccessPointStatistics)
		if err != nil {
			lastErr = err
			log.Errorf("problem with getting stats from BIND 9, bad access statistics point: %+v", err)
			continue
		}
		address := storkutil.HostWithPortURL(sap.Address, sap.Port)
		path := "json/v1"
		url := fmt.Sprintf("%s%s", address, path)
		httpRsp, err := pbe.HTTPClient.Call(url, bytes.NewBuffer([]byte(request)))
		if err != nil {
			lastErr = err
			log.Errorf("problem with getting stats from BIND 9: %+v", err)
			continue
		}
		body, err := ioutil.ReadAll(httpRsp.Body)
		httpRsp.Body.Close()
		if err != nil {
			lastErr = err
			log.Errorf("problem with reading stats response from BIND 9: %+v", err)
			continue
		}

		// parse response
		var rspIfc interface{}
		response := string(body)
		err = json.Unmarshal([]byte(response), &rspIfc)
		if err != nil {
			lastErr = err
			log.Errorf("failed to parse responses from BIND 9: %s", err)
			continue
		}

		err = pbe.setDaemonStats(rspIfc)
		if err != nil {
			log.Errorf("cannot get stat from daemon: %+v", err)
		}
	}

	return lastErr
}

// initViewStats initializes the maps for storing metrics.
func (pbe *PromBind9Exporter) initViewStats(viewName string) {
	_, ok := pbe.stats.Views[viewName]
	if !ok {
		resolverCachestats := make(map[string]float64)

		pbe.stats.Views[viewName] = PromBind9ViewStats{
			ResolverCachestats: resolverCachestats,
		}
	}
}
