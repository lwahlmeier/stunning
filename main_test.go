package main // import "github.com/lwahlmeier/stunning"
import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lwahlmeier/stunlib"
	"github.com/stretchr/testify/assert"
)

func TestBadStunReaderLoop(t *testing.T) {
	rc := make(chan *StunRead, 500)
	endLoop := make(chan bool)
	go poolWaiter(rc, endLoop)
	uw := &UDPWriterSt{bufferChan: make(chan []byte)}
	rc <- &StunRead{conn: uw, buffer: []byte{}, addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5555}, readTime: time.Now()}
	select {
	case buff := <-uw.bufferChan:
		fmt.Println(buff)
		assert.Fail(t, "Should not get a buffer back here")
	case <-time.After(time.Millisecond * 100):
		endLoop <- true
		<-time.After(time.Millisecond * 100)
	}
}

func TestGoodStunReaderLoop(t *testing.T) {
	rc := make(chan *StunRead, 500)
	endLoop := make(chan bool)
	go poolWaiter(rc, endLoop)
	sp := stunlib.NewStunPacketBuilder().Build()
	ad := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 5555}
	uw := &UDPWriterSt{bufferChan: make(chan []byte)}
	rc <- &StunRead{conn: uw, buffer: sp.GetBytes(), addr: ad, readTime: time.Now()}
	select {
	case buff := <-uw.bufferChan:
		nsp, err := stunlib.NewStunPacket(buff)
		assert.True(t, err == nil)
		nad, err := nsp.GetAddress()
		assert.True(t, err == nil)
		assert.Equal(t, ad, nad)
		endLoop <- true
	case <-time.After(time.Millisecond * 100):
		assert.Fail(t, "Should not get a buffer back here")
		<-time.After(time.Millisecond * 100)
	}
}

type UDPWriterSt struct {
	bufferChan chan []byte
}

func (uw *UDPWriterSt) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	uw.bufferChan <- b
	return len(b), nil
}
