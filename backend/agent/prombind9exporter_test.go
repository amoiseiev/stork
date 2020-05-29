package agent

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

// Fake app monitor that returns some predefined list of apps.
type PromFakeBind9AppMonitor struct {
	Apps []*App
}

func (fam *PromFakeBind9AppMonitor) GetApps() []*App {
	log.Println("GetApps")
	accessPoints := makeAccessPoint(AccessPointStatistics, "1.2.3.4", "", 1234)
	accessPoints = append(accessPoints, AccessPoint{
		Type:    AccessPointControl,
		Address: "1.9.5.3",
		Port:    1953,
		Key:     "abcd",
	})
	return []*App{{
		Type:         AppTypeBind9,
		AccessPoints: accessPoints,
	}}
}

func (fam *PromFakeBind9AppMonitor) Shutdown() {
}

// Check creating PromBind9Exporter, check if prometheus stats are set up.
func TestNewPromBind9ExporterBasic(t *testing.T) {
	fam := &PromFakeBind9AppMonitor{}
	pbe := NewPromBind9Exporter(fam)
	defer pbe.Shutdown()

	require.NotNil(t, pbe.HTTPClient)
	require.NotNil(t, pbe.HTTPServer)
	require.Len(t, pbe.serverStatsDesc, 4)
	require.Len(t, pbe.viewStatsDesc, 6)
}

// Check starting PromBind9Exporter and collecting stats.
func TestPromBind9ExporterStart(t *testing.T) {
	defer gock.Off()
	gock.New("http://1.2.3.4:1234/").
		Post("/").
		Persist().
		Reply(200).
		BodyString(`{ "json-stats-version": "1.2",
                              "boot-time": "2020-04-21T07:13:08.888Z",
                              "config-time": "2020-04-21T07:13:09.989Z",
                              "current-time": "2020-04-21T07:19:28.258Z",
                              "version":"9.16.2",
                              "qtypes": {
                                  "A": 201,
                                  "AAAA": 200,
                                  "DNSKEY": 53
                              },
			      "views": {
                                "_default": {
                                  "resolver": {
                                    "cachestats": {
                                      "CacheHits": 40,
                                      "CacheMisses": 10,
                                      "QueryHits": 30,
                                      "QueryMisses": 20
                                    }
                                  }
                                }
                              }
                            }`)
	fam := &PromFakeBind9AppMonitor{}
	pbe := NewPromBind9Exporter(fam)
	defer pbe.Shutdown()

	gock.InterceptClient(pbe.HTTPClient.client)

	// prepare sane settings
	pbe.Settings.Port = 1234
	pbe.Settings.Interval = 1 // 1 second

	// start exporter
	pbe.Start()

	// boot_time_seconds
	expect, _ := time.Parse(time.RFC3339, "2020-04-21T07:13:08.888Z")
	require.EqualValues(t, expect, pbe.stats.BootTime)
	// config_time_seconds
	expect, _ = time.Parse(time.RFC3339, "2020-04-21T07:13:09.989Z")
	require.EqualValues(t, expect, pbe.stats.ConfigTime)
	// current_time_seconds
	expect, _ = time.Parse(time.RFC3339, "2020-04-21T07:19:28.258Z")
	require.EqualValues(t, expect, pbe.stats.CurrentTime)

	// incoming_queries_total
	require.EqualValues(t, 201.0, pbe.stats.IncomingQueries["A"])
	require.EqualValues(t, 200.0, pbe.stats.IncomingQueries["AAAA"])
	require.EqualValues(t, 53.0, pbe.stats.IncomingQueries["DNSKEY"])

	// resolver_cache_hit_ratio
	require.EqualValues(t, 0.8, pbe.stats.Views["_default"].ResolverCachestats["CacheHitRatio"])
	// resolver_cache_hits
	require.EqualValues(t, 40.0, pbe.stats.Views["_default"].ResolverCachestats["CacheHits"])
	// resolver_cache_misses
	require.EqualValues(t, 10.0, pbe.stats.Views["_default"].ResolverCachestats["CacheMisses"])
	// resolver_query_hit_ratio
	require.EqualValues(t, 0.6, pbe.stats.Views["_default"].ResolverCachestats["QueryHitRatio"])
	// resolver_query_hits
	require.EqualValues(t, 30.0, pbe.stats.Views["_default"].ResolverCachestats["QueryHits"])
	// resolver_query_misses
	require.EqualValues(t, 20.0, pbe.stats.Views["_default"].ResolverCachestats["QueryMisses"])
}
