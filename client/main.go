package main // import "github.com/lwahlmeier/stunning/client"

import (
	"math/rand"
	"net"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PremiereGlobal/stim/pkg/stimlog"
	sets "github.com/deckarep/golang-set"
	"github.com/lwahlmeier/easyatomics"
	"github.com/lwahlmeier/stunlib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var log = stimlog.GetLogger()
var config *viper.Viper
var version string
var errors = easyatomics.AtomicUint64{}
var latency = easyatomics.AtomicInt64{}
var success = easyatomics.AtomicInt64{}

func main() {

	config = viper.New()
	config.SetEnvPrefix("stun_client")
	config.AutomaticEnv()
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	if version == "" || version == "latest" {
		version = "unknown"
	}

	var cmd = &cobra.Command{
		Use:   "stun_client",
		Short: "run the stun_client",
		Long:  "run the stun_client",
		Run:   cMain,
	}
	cmd.PersistentFlags().Int("requests", 1, "number of requests to send per client")
	config.BindPFlag("requests", cmd.PersistentFlags().Lookup("requests"))

	cmd.PersistentFlags().Int("clients", 1, "number of clients to run in parallel")
	config.BindPFlag("clients", cmd.PersistentFlags().Lookup("clients"))

	cmd.PersistentFlags().Int("timeout", 500, "number of milliseconds to wait for a response to timeout")
	config.BindPFlag("timeout", cmd.PersistentFlags().Lookup("timeout"))

	cmd.PersistentFlags().String("addr", "stun.l.google.com:19302", "stun server to use")
	config.BindPFlag("addr", cmd.PersistentFlags().Lookup("addr"))

	cmd.PersistentFlags().String("loglevel", "info", "level to show logs at (warn, info, debug, trace)")
	config.BindPFlag("loglevel", cmd.PersistentFlags().Lookup("loglevel"))

	cmd.Execute()

}

func cMain(cmd *cobra.Command, args []string) {
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

	var wait sync.WaitGroup
	r := config.GetInt("requests")
	c := config.GetInt("clients")
	to := config.GetInt("timeout")
	addr := config.GetString("addr")
	td := timeData{data: make([]time.Duration, 0), lock: &sync.Mutex{}, sps: sets.NewSet()}
	for i := 0; i < c; i++ {
		go runClient(r, to, addr, &wait, &td)
		wait.Add(1)
	}
	wait.Wait()
	ns := time.Nanosecond * time.Duration(latency.Get())
	log.Info("total request time:\t{}", ns)
	log.Info("latency:\t\t{}", time.Duration(ns.Nanoseconds()/success.Get()))
	log.Info("success:\t\t{}", success.Get())
	log.Info("errors:\t\t\t{}", errors.Get())
	td.sort()
	tda := td.get()
	if len(tda) > 1 {
		log.Info("min:\t\t\t{}", tda[0])
		log.Info("max:\t\t\t{}", tda[len(tda)-1])
	}
	addrs := td.getAddrs()
	for _, v := range addrs {
		log.Info("Got IP:\t\t\t{}", v)
	}
}

func runClient(reqs, to int, addr string, wait *sync.WaitGroup, td *timeData) {
	s, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	CheckError(err)
	conn, err := net.ListenUDP("udp", s)
	CheckError(err)
	host, port, err := net.SplitHostPort(addr)
	CheckError(err)
	nport, err := strconv.Atoi(port)
	CheckError(err)
	ips, err := net.LookupIP(host)
	CheckError(err)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	for r := 0; r < reqs; r++ {
		ip := ips[rand.Intn(len(ips))]
		remoteAddr := &net.UDPAddr{IP: ip, Port: nport}
		log.Debug("Checking stun for host: '{}:{}'", ip, nport)
		ba := make([]byte, 1500)
		sp := stunlib.NewStunPacketBuilder().SetStunMessage(stunlib.SMRequest).Build()
		startT := time.Now()
		conn.WriteToUDP(sp.GetBytes(), remoteAddr)
		n, ua, err := conn.ReadFrom(ba)
		if err != nil {
			log.Warn("{}", err)
			errors.Inc()
			continue
		}
		spr, err := stunlib.NewStunPacket(ba[:n])
		if err != nil {
			log.Warn("{}", err)
			errors.Inc()
			continue
		}
		na, err := spr.GetAddress()
		if err != nil {
			log.Warn("{}", err)
			errors.Inc()
			continue
		}
		td.addAddr(na)
		since := time.Since(startT)
		td.add(since)
		latency.IncBy(since.Nanoseconds())
		success.Inc()
		log.Debug("SPR:{}, {}=>{}=>{}, delay:{}", spr.GetStunMessageType(), conn.LocalAddr(), ua, na, since)
	}
	wait.Done()
}

func CheckError(err error) {
	if err != nil {
		debug.PrintStack()
		log.Fatal("Fatal Error, Exiting!:{}", err)
	}
}

type timeData struct {
	data []time.Duration
	sps  sets.Set
	lock *sync.Mutex
}

func (d *timeData) getAddrs() []string {
	al := make([]string, 0)
	d.sps.Each(func(i interface{}) bool {
		al = append(al, i.(string))
		return false
	})
	return al
}

func (d *timeData) addAddr(addr *net.UDPAddr) {
	d.sps.Add(addr.String())
}

func (d *timeData) add(td time.Duration) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.data = append(d.data, td)
}

func (d *timeData) sort() {
	d.lock.Lock()
	defer d.lock.Unlock()
	sort.Slice(d.data, func(i, j int) bool { return d.data[i] < d.data[j] })
}

func (d *timeData) getAvg() time.Duration {
	d.lock.Lock()
	defer d.lock.Unlock()
	x := int64(0)
	for _, td := range d.data {
		x += td.Nanoseconds()
	}
	x = x / int64(len(d.data))
	return time.Duration(x)
}

func (d *timeData) get() []time.Duration {
	d.lock.Lock()
	defer d.lock.Unlock()
	ntd := make([]time.Duration, len(d.data))
	copy(ntd, d.data)
	return ntd
}
