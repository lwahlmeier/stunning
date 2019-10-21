// Package stunlib provides an api for parsing and genrating stun packets
package stunlib // import "github.com/lwahlmeier/stunlib"

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
)

func init() {
	X := make([]byte, 8)
	crypto_rand.Read(X)
	rand.Seed(int64(binary.BigEndian.Uint64(X)))
}

type StunMessage uint16
type StunAttribute uint16

const (
	SMRequest    StunMessage = 0x0001
	SMSuccess    StunMessage = 0x0101
	SMFailure    StunMessage = 0x0111
	SMIndication StunMessage = 0x0011

	SAMappedAddress    StunAttribute = 0x0001
	SAResponseAddress  StunAttribute = 0x0002
	SAChangeRequest    StunAttribute = 0x0003
	SASourceAddress    StunAttribute = 0x0004
	SAChangedRequest   StunAttribute = 0x0005
	SAUsername         StunAttribute = 0x0006
	SAPassword         StunAttribute = 0x0007
	SAMessageIntegrity StunAttribute = 0x0008
	SAErrorCode        StunAttribute = 0x0009

	SAUnknownAttribute StunAttribute = 0x000a
	SAReflectedFrom    StunAttribute = 0x000b

	SARealm StunAttribute = 0x0014
	SANonce StunAttribute = 0x0015

	SAXORMappedAddress StunAttribute = 0x0020
	SAPriority         StunAttribute = 0x0024
	SAUseCandidate     StunAttribute = 0x0025

	SASoftware        StunAttribute = 0x8022
	SAAlternateServer StunAttribute = 0x8023
	SAFingerPrint     StunAttribute = 0x8028
	SAIceControlled   StunAttribute = 0x8029
	SAIceControlling  StunAttribute = 0x802a
)

func SAOptional(sa StunAttribute) bool {
	return sa&0x8000 == 0
}

type TransactionID struct {
	tid []byte
}

//NewTID will create a new TransactionID from the provided []byte
//The []byte must be only 12 bytes
func NewTID(ba []byte) (*TransactionID, error) {
	if len(ba) != 12 {
		return nil, errors.New("Invalid TransactionID!")
	}
	return &TransactionID{tid: ba}, nil
}

//CreateTID will create a new Random TransactionID
func CreateTID() *TransactionID {
	ba := make([]byte, 12)
	rand.Read(ba)
	return &TransactionID{tid: ba}
}

//UnMaskAddress uses this TransactionID to unmask an SAXORMappedAddress
func (tid *TransactionID) UnMaskAddress(addr []byte) *net.UDPAddr {
	return UnMaskAddress(*tid, addr)
}

//MaskAddress takes an net.UDPAddr and turns it into a SAXORMappedAddress []byte
func (tid *TransactionID) MaskAddress(addr *net.UDPAddr) []byte {
	return CreateMaskedAddress(*tid, addr)
}

//GetTID provides this TransactionIDs []byte
func (tid *TransactionID) GetTID() []byte {
	return tid.tid
}

//This prints the TransactionID as a HexString
func (tid *TransactionID) String() string {
	return fmt.Sprintf("%X", tid.tid)
}

type StunPacket struct {
	buffer []byte
}

//NewStunPacket create a new StunPacket from the provided []byte
func NewStunPacket(b []byte) (*StunPacket, error) {
	if !IsStunPacket(b) {
		return nil, errors.New("Not a valid stun packet!")
	}
	return &StunPacket{buffer: b}, nil
}

//GetStunMessageType will return the current StunMessage for this StunPacket
func (sp *StunPacket) GetStunMessageType() StunMessage {
	return StunMessage(binary.BigEndian.Uint16(sp.buffer[:2]))
}

//GetAllAttributes will return all the StunAttributes in this StunPacket as an Array
func (sp *StunPacket) GetAllAttributes() []StunAttribute {
	sas := make([]StunAttribute, 0)
	pos := 20
	for pos < len(sp.buffer) {
		t := binary.BigEndian.Uint16(sp.buffer[pos : pos+2])
		s := int(binary.BigEndian.Uint16(sp.buffer[pos+2 : pos+4]))
		sas = append(sas, StunAttribute(t))
		pos = ((pos + s + 4 + 3) & ^3)
	}
	return sas
}

//GetAttribute returns the []byte for a given StunAttribute
func (sp *StunPacket) GetAttribute(sa StunAttribute) []byte {
	pos := 20
	for pos < len(sp.buffer) {
		t := StunAttribute(binary.BigEndian.Uint16(sp.buffer[pos : pos+2]))
		s := int(binary.BigEndian.Uint16(sp.buffer[pos+2 : pos+4]))
		if t == sa {
			return sp.buffer[pos+4 : pos+4+s]
		}
		pos = ((pos + s + 4 + 3) & ^3)
	}
	return nil
}

//GetTxID returns the TransactionID for this StunPacket
func (sp *StunPacket) GetTxID() *TransactionID {
	return &TransactionID{tid: sp.buffer[8:20]}
}

//ToBuilder creates a StunPacketBuilder with this StunPackets Data as the base
func (sp *StunPacket) ToBuilder() *StunPacketBuilder {
	return fromStunPacket(sp)
}

//HasAddress returns true if this StunPacket has a MappedAddress of any kind.
func (sp *StunPacket) HasAddress() bool {
	sas := sp.GetAllAttributes()
	for _, sa := range sas {
		sab := StunAttribute(sa)
		if sab == SAXORMappedAddress || sab == SAMappedAddress {
			return true
		}
	}
	return false
}

func (sp *StunPacket) HasFingerPrint() bool {
	sas := sp.GetAllAttributes()
	for _, sa := range sas {
		sab := StunAttribute(sa)
		if sab == SAFingerPrint {
			return true
		}
	}
	return false
}

//GetBytes gets the underliying []byte for this StunPacket
func (sp *StunPacket) GetBytes() []byte {
	return sp.buffer
}

//GetAddress returns the MappedAddress or XORMappedAddress in this StunPacket if its exits
func (sp *StunPacket) GetAddress() (*net.UDPAddr, error) {
	var ip net.IP
	var port uint16
	sas := sp.GetAttribute(SAMappedAddress)
	if len(sas) == 0 {
		sas = sp.GetAttribute(SAXORMappedAddress)
		if len(sas) == 0 {
			return nil, errors.New("MappedAddress Not found!")
		}
		return UnMaskAddress(*sp.GetTxID(), sas), nil
	} else {
		ip = net.IP(sas[4:])
		port = (binary.BigEndian.Uint16(sas[2:4]))
		return &net.UDPAddr{IP: ip, Port: int(port)}, nil
	}

}

type StunPacketBuilder struct {
	mt            StunMessage
	tid           *TransactionID
	attribs       []StunAttribute
	attribsBuffer [][]byte
	padding       byte
	fingerprint   bool
	key           []byte
}

func fromStunPacket(sp *StunPacket) *StunPacketBuilder {
	spb := NewStunPacketBuilder()
	spb.SetStunMessage(sp.GetStunMessageType())
	spb.SetTXID(sp.GetTxID())
	for _, sa := range sp.GetAllAttributes() {
		ba := sp.GetAttribute(sa)
		spb.SetAttribue(sa, ba)
	}
	return spb
}

func NewStunPacketBuilder() *StunPacketBuilder {
	return &StunPacketBuilder{
		mt:            SMRequest,
		tid:           nil,
		attribs:       make([]StunAttribute, 0),
		attribsBuffer: make([][]byte, 0),
		padding:       0x00,
		fingerprint:   false,
		key:           nil,
	}
}

func (spb *StunPacketBuilder) SetStunMessage(sm StunMessage) *StunPacketBuilder {
	spb.mt = sm
	return spb
}

func (spb *StunPacketBuilder) SetTXID(txid *TransactionID) *StunPacketBuilder {
	spb.tid = txid
	return spb
}

func (spb *StunPacketBuilder) SetAttribue(sa StunAttribute, ba []byte) *StunPacketBuilder {
	spb.attribs = append(spb.attribs, sa)
	spb.attribsBuffer = append(spb.attribsBuffer, ba)
	return spb
}

func (spb *StunPacketBuilder) SetPaddingByte(b byte) *StunPacketBuilder {
	spb.padding = b
	return spb
}

func (spb *StunPacketBuilder) SetAddress(ua *net.UDPAddr) *StunPacketBuilder {
	ip := ua.IP.To4()
	if ip == nil {
		ip = ua.IP
	}
	aba := []byte(ip)
	var saba []byte
	if len(aba) == 4 {
		saba = make([]byte, 8)
		saba[1] = 1
	} else {
		saba = make([]byte, 20)
		saba[1] = 2
	}
	binary.BigEndian.PutUint16(saba[2:4], uint16(ua.Port))
	copy(saba[4:], aba)
	spb.SetAttribue(SAMappedAddress, saba)
	return spb
}

func (spb *StunPacketBuilder) SetXORAddress(ua *net.UDPAddr) *StunPacketBuilder {
	ip := ua.IP.To4()
	if ip == nil {
		ip = ua.IP
	}
	if spb.tid == nil {
		spb.tid = CreateTID()
	}
	spb.SetAttribue(SAXORMappedAddress, CreateMaskedAddress(*spb.tid, ua))
	return spb
}

func (spb *StunPacketBuilder) ClearAttributes() *StunPacketBuilder {
	spb.attribs = make([]StunAttribute, 0)
	spb.attribsBuffer = make([][]byte, 0)
	return spb
}

func (spb *StunPacketBuilder) AddFingerprint(fp bool) *StunPacketBuilder {
	spb.fingerprint = fp
	return spb
}

func (spb *StunPacketBuilder) Build() *StunPacket {
	size := 20
	for _, v := range spb.attribsBuffer {
		size += len(v) + 4
		size = (size + 3) & ^3
	}
	if spb.fingerprint {
		size += 8
	}
	ba := make([]byte, size)
	binary.BigEndian.PutUint16(ba[:2], uint16(spb.mt))
	binary.BigEndian.PutUint16(ba[2:4], uint16(size-20))
	binary.BigEndian.PutUint32(ba[4:8], stunMagic)
	if spb.tid == nil {
		copy(ba[8:20], CreateTID().GetTID())
	} else {
		copy(ba[8:20], spb.tid.GetTID())
	}
	pos := 20
	for i, sa := range spb.attribs {
		sab := spb.attribsBuffer[i]
		bl := len(sab)
		binary.BigEndian.PutUint16(ba[pos:pos+2], uint16(sa))
		binary.BigEndian.PutUint16(ba[pos+2:pos+4], uint16(bl))
		pos += 4
		copy(ba[pos:pos+bl], sab)
		pos += bl
		for pos&3 != 0 {
			ba[pos] = spb.padding
			pos++
		}
	}
	if spb.fingerprint {
		fps := size - 8
		fp := CreateStunFingerPrint(ba[:fps])
		binary.BigEndian.PutUint16(ba[fps:fps+2], uint16(SAFingerPrint))
		binary.BigEndian.PutUint16(ba[fps+2:fps+4], uint16(4))
		binary.BigEndian.PutUint32(ba[fps+4:fps+8], fp)
	}
	sp, _ := NewStunPacket(ba)
	return sp
}
