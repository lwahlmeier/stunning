package main // import "github.com/lwahlmeier/stunning"

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/PremiereGlobal/stim/pkg/stimlog"
	"github.com/lwahlmeier/stunlib"
	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var log = stimlog.GetLogger()
var version string
var config *viper.Viper
var stunRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "stun_requests_total",
	Help: "The number of total stun requests",
})

var nonStunPackets = promauto.NewCounter(prometheus.CounterOpts{
	Name: "stun_non_stun_packets",
	Help: "The total number non-stun packets received",
})

var stunBytesReceived = promauto.NewCounter(prometheus.CounterOpts{
	Name: "stun_bytes_received",
	Help: "The total number of bytes received",
})

var stunBytesSent = promauto.NewCounter(prometheus.CounterOpts{
	Name: "stun_bytes_sent",
	Help: "The total number of bytes sent",
})

var stunProcessLatency = promauto.NewHistogram(prometheus.HistogramOpts{
	Name: "stun_response_latency_seconds",
	Help: "The time it takes to process a single stun request in seconds",
})

var stunQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "stun_queue_size",
	Help: "The current size of the stun queue",
})

var stunGeoIPLatency = promauto.NewHistogram(prometheus.HistogramOpts{
	Name: "stun_geoip_latency_seconds",
	Help: "The time it takes to lookup a GeoIP",
})

var stunGeoIPQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "stun_geoip_queue_size",
	Help: "The current size of the geoIP queue",
})

func main() {
	var err error

	config = viper.New()
	config.SetEnvPrefix("stunning")
	config.AutomaticEnv()
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	if version == "" || version == "latest" {
		version = "unknown"
	}

	var cmd = &cobra.Command{
		Use:   "stunning",
		Short: "launch stunning service",
		Long:  "launch stunning service",
		Run:   cMain,
	}

	cmd.PersistentFlags().String("loglevel", "info", "level to show logs at (warn, info, debug, trace)")
	config.BindPFlag("loglevel", cmd.PersistentFlags().Lookup("loglevel"))

	cmd.PersistentFlags().String("stunAddress", "0.0.0.0:3478", "The ip:port to listen for stun requests on, this can be a list (comma sperated '127.0.0.1:1122,192.168.2.24:6543')")
	config.BindPFlag("stunAddress", cmd.PersistentFlags().Lookup("stunAddress"))

	cmd.PersistentFlags().String("metricsAddress", "0.0.0.0:8080", "The ip:port to listen for prometheus metrics requests on, only accepts a single address ('127.0.0.1:8080')")
	config.BindPFlag("metricsAddress", cmd.PersistentFlags().Lookup("metricsAddress"))

	//TODO:
	// cmd.PersistentFlags().Int("stunThrottle", 2000, "Sets the throttle rate for stun requests per client")
	// config.BindPFlag("stunThrottle", cmd.PersistentFlags().Lookup("stunThrottle"))

	cmd.PersistentFlags().Int("poolSize", 20, "Max pool size (go routines) to use for processing requests")
	config.BindPFlag("poolSize", cmd.PersistentFlags().Lookup("poolSize"))

	cmd.PersistentFlags().Bool("fingerPrint", false, "Should stun messages add fingerPrints")
	config.BindPFlag("fingerPrint", cmd.PersistentFlags().Lookup("fingerPrint"))

	cmd.PersistentFlags().Bool("logGeoIP", false, "set to true if we should log GEO IP data (geoIPPath must also be set!)")
	config.BindPFlag("logGeoIP", cmd.PersistentFlags().Lookup("logGeoIP"))
	cmd.PersistentFlags().String("geoIPPath", "/tmp/none", "Path to use for GeoIPDB lookups")
	config.BindPFlag("geoIPPath", cmd.PersistentFlags().Lookup("geoIPPath"))

	err = cmd.Execute()
	CheckError(err)
}

func cMain(cmd *cobra.Command, args []string) {
	if config.GetBool("version") {
		fmt.Printf("%s\n", version)
		os.Exit(0)
	}
	var ll stimlog.Level
	switch strings.ToLower(config.GetString("loglevel")) {
	case "info":
		ll = stimlog.InfoLevel
	case "warn":
		ll = stimlog.WarnLevel
	case "debug":
		ll = stimlog.DebugLevel
	case "trace":
		ll = stimlog.TraceLevel
	}
	stimlog.GetLoggerConfig().SetLevel(ll)
	ps := config.GetInt("poolSize")
	stunAddress := strings.Split(config.GetString("stunAddress"), ",")
	metricsAddress := config.GetString("metricsAddress")
	logGeoIP := config.GetBool("logGeoIP")
	stunServers := make([]*StunServer, 0)
	rc := make(chan *StunRead, 500)
	endLoop := make(chan bool)
	ipc := make(chan net.IP, 500)
	fp := config.GetBool("fingerPrint")
	log.Info("Stun Addresses:{}", stunAddress)
	log.Info("Metrics Address:{}", metricsAddress)
	log.Info("PoolSize:{}", ps)
	log.Info("GeoIP logging:{}", logGeoIP)
	log.Info("FingerPrint: {}", fp)
	for _, v := range stunAddress {
		s, err := net.ResolveUDPAddr("udp", v)
		CheckError(err)
		conn, err := net.ListenUDP("udp", s)
		CheckError(err)
		stunServers = append(stunServers, &StunServer{readChan: rc, conn: conn, geoIPChan: ipc, geoIPLog: logGeoIP, fingerPrint: fp})
	}
	for i := 0; i < ps; i++ {
		go poolWaiter(rc, endLoop)
	}
	if logGeoIP {
		log.Info("GeoIP DB Path:{}", config.GetString("geoIPPath"))
		go doGeoIP(config.GetString("geoIPPath"), ipc)
	}
	for _, s := range stunServers {
		go stunListener(s)
	}
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(metricsAddress, nil)
	if err != nil {
		log.Fatal("{}", err)
	}
}

func CheckError(err error) {
	if err != nil {
		debug.PrintStack()
		log.Fatal("Fatal Error, Exiting!:{}", err)
	}
}

func stunListener(ss *StunServer) {
	for {
		ba := make([]byte, 1500)
		l, addr, err := ss.conn.ReadFromUDP(ba)
		if err != nil {
			log.Warn("Got Generic UDP ERROR:{}", err)
			continue
		}
		log.Debug("Got UDP Packet from:{}", addr)
		stunBytesReceived.Add(float64(l))
		ss.readChan <- &StunRead{conn: ss.conn, buffer: ba[:l], addr: addr, readTime: time.Now(), geoIPLog: ss.geoIPLog, geoIPChan: ss.geoIPChan, fingerPrint: ss.fingerPrint}
	}
}

func poolWaiter(reader chan *StunRead, endloop chan bool) {
	for {
		select {
		case sr := <-reader:
			{
				log.Debug("Packet from:{}, {}", sr.addr, fmt.Sprintf("%02X", sr.buffer))
				sp, err := stunlib.NewStunPacket(sr.buffer)
				if err != nil {
					nonStunPackets.Inc()
					log.Debug("Got Invalid Stun Packet from:{}, {}", sr.addr, err)
					continue
				}
				log.Debug("Got Stun Packet from:{}", sr.addr)
				stunRequestsTotal.Inc()
				rsp := stunlib.NewStunPacketBuilder().SetStunMessage(stunlib.SMSuccess).SetTXID(sp.GetTxID()).SetXORAddress(sr.addr).SetAddress(sr.addr).AddFingerprint(sr.fingerPrint).Build()
				l, _ := sr.conn.WriteToUDP(rsp.GetBytes(), sr.addr)
				if l == len(rsp.GetBytes()) {
					stunProcessLatency.Observe(time.Since(sr.readTime).Seconds())
					stunBytesSent.Add(float64(l))
					if sr.geoIPLog {
						sr.geoIPChan <- sr.addr.IP
						stunGeoIPQueueSize.Set(float64(len(sr.geoIPChan)))
					}
					stunQueueSize.Set(float64(len(reader)))
				}
			}
		case done := <-endloop:
			log.Info("Got Loop Exit: {}", done)
			return
		}
	}
}

func doGeoIP(geoPath string, ipc chan net.IP) {
	db, err := geoip2.Open(geoPath)
	CheckError(err)
	for {
		ip := <-ipc
		start := time.Now()
		record, err := db.City(ip)
		if err != nil {
			log.Warn("{}", err)
		} else {
			log.Info("IP:{} Coordinates: {}, {}", ip.String(), record.Location.Latitude, record.Location.Longitude)
			stunGeoIPLatency.Observe(time.Since(start).Seconds())
		}
		stunGeoIPQueueSize.Set(float64(len(ipc)))
	}
}

type StunServer struct {
	conn        *net.UDPConn
	readChan    chan *StunRead
	geoIPLog    bool
	geoIPChan   chan net.IP
	fingerPrint bool
}

type StunRead struct {
	conn        UDPWriter
	addr        *net.UDPAddr
	buffer      []byte
	readTime    time.Time
	geoIPLog    bool
	geoIPChan   chan net.IP
	fingerPrint bool
}

type UDPWriter interface {
	WriteToUDP(b []byte, addr *net.UDPAddr) (int, error)
}
