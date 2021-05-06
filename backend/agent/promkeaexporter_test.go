package agent

import (
	"flag"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"gopkg.in/h2non/gock.v1"
)

// Fake app monitor that returns some predefined list of apps.
type PromFakeAppMonitor struct {
	Apps []App
}

func (fam *PromFakeAppMonitor) GetApps() []App {
	log.Println("GetApps")
	ka := &KeaApp{
		BaseApp: BaseApp{
			Type:         AppTypeKea,
			AccessPoints: makeAccessPoint(AccessPointControl, "0.1.2.3", "", 1234),
		},
		HTTPClient: nil,
	}
	return []App{ka}
}

func (fam *PromFakeAppMonitor) GetApp(appType, apType, address string, port int64) App {
	return nil
}

func (fam *PromFakeAppMonitor) Shutdown() {
}

func (fam *PromFakeAppMonitor) Start(storkAgent *StorkAgent) {
}

// Check creating PromKeaExporter, check if prometheus stats are set up.
func TestNewPromKeaExporterBasic(t *testing.T) {
	fam := &PromFakeAppMonitor{}
	var settings cli.Context
	pke := NewPromKeaExporter(&settings, fam)
	defer pke.Shutdown()

	require.NotNil(t, pke.HTTPClient)
	require.NotNil(t, pke.HTTPServer)

	require.Len(t, pke.PktStatsMap, 31)
	require.Len(t, pke.Adr4StatsMap, 6)
	require.Len(t, pke.Adr6StatsMap, 9)
}

// Check starting PromKeaExporter and collecting stats.
func TestPromKeaExporterStart(t *testing.T) {
	defer gock.Off()
	gock.New("http://0.1.2.3:1234/").
		Post("/").
		Persist().
		Reply(200).
		BodyString(`[{"result":0, "arguments": {
                    "subnet[7].assigned-addresses": [ [ 13, "2019-07-30 10:04:28.386740" ] ],
                    "pkt4-nak-received": [ [ 19, "2019-07-30 10:04:28.386733" ] ]
                }}]`)

	fam := &PromFakeAppMonitor{}
	flags := flag.NewFlagSet("test", 0)
	flags.Int("prometheus-kea-exporter-port", 9547, "usage")
	flags.Int("prometheus-kea-exporter-interval", 10, "usage")
	settings := cli.NewContext(nil, flags, nil)
	settings.Set("prometheus-kea-exporter-port", "1234")
	settings.Set("prometheus-kea-exporter-interval", "1")
	pke := NewPromKeaExporter(settings, fam)
	defer pke.Shutdown()

	gock.InterceptClient(pke.HTTPClient.client)

	// start exporter
	pke.Start()
	require.NotNil(t, pke.Ticker)

	// wait 1.5 seconds that collecting is invoked at least once
	time.Sleep(1500 * time.Millisecond)

	// check if assigned-addresses is 13
	metric, _ := pke.Adr4StatsMap["assigned-addresses"].GetMetricWith(prometheus.Labels{"subnet": "7"})
	require.Equal(t, 13.0, testutil.ToFloat64(metric))

	// check if pkt4-nak-received is 19
	metric, _ = pke.PktStatsMap["pkt4-nak-received"].Stat.GetMetricWith(prometheus.Labels{"operation": "nak"})
	require.Equal(t, 19.0, testutil.ToFloat64(metric))
}
