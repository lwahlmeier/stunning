package stunlib // import "github.com/lwahlmeier/stunlib"

import (
	"encoding/binary"
	"hash/crc32"
	"net"
)

const (
	stunMagic            = 0x2112a442
	stunShortMagic       = 0x2112
	stunFingerPrintMagic = 0x5354554e
)

//CreateStunFingerPrint adds s fingerprint to a StunPacket
//Be very carful with this method the size stun attr need to include the
//The 8 bytes that will be added as the fingerprint, but the []byte
//Can not have those bytes
func CreateStunFingerPrint(sp []byte) uint32 {
	return stunFingerPrintMagic ^ crc32.ChecksumIEEE(sp)
}

//VerifyFingerPrint verifies there is a fingerprint and it is correct.
func VerifyFingerPrint(sp StunPacket) bool {
	ba := sp.GetBytes()
	size := len(ba)
	sizep := size - 8
	sa := binary.BigEndian.Uint16(ba[size-8 : size-6])
	if StunAttribute(sa) != SAFingerPrint {
		return false
	}
	crc32 := stunFingerPrintMagic ^ crc32.ChecksumIEEE(ba[:sizep])
	oc := binary.BigEndian.Uint32(ba[size-4:])
	if crc32 == oc {
		return true
	}
	return false
}

func UnmaskIP(tid TransactionID, address []byte) net.IP {
	na := make([]byte, len(address))
	na[0] = (byte)(address[0] ^ 0x21)
	na[1] = (byte)(address[1] ^ 0x12)
	na[2] = (byte)(address[2] ^ 0xa4)
	na[3] = (byte)(address[3] ^ 0x42)
	tidbb := tid.GetTID()
	for i := 4; i < len(na); i++ {
		na[i] = address[i] ^ tidbb[i-4]
	}
	return net.IP(na)
}

func UnMaskAddress(tid TransactionID, mba []byte) *net.UDPAddr {
	ip := UnmaskIP(tid, mba[4:])
	port := (binary.BigEndian.Uint16(mba[2:4]) ^ stunShortMagic)
	return &net.UDPAddr{IP: ip, Port: int(port)}
}

func CreateMaskedAddress(tid TransactionID, ua *net.UDPAddr) []byte {
	ip := ua.IP.To4()
	if ip == nil {
		ip = ua.IP
	}
	ipl := len(ip)
	to := make([]byte, ipl+4)
	if ipl == 4 {
		to[1] = 1
	} else {
		to[1] = 2
	}
	to[4] = (byte)(ip[0] ^ 0x21)
	to[5] = (byte)(ip[1] ^ 0x12)
	to[6] = (byte)(ip[2] ^ 0xa4)
	to[7] = (byte)(ip[3] ^ 0x42)
	tidbb := tid.GetTID()
	for i := 4; i < ipl; i++ {
		to[i+4] = ip[i] ^ tidbb[i-4]
	}
	binary.BigEndian.PutUint16(to[2:4], uint16(ua.Port)^stunShortMagic)

	return to
}

func IsStunPacket(ba []byte) bool {
	if len(ba) < 20 {
		return false
	}
	sm := StunMessage(binary.BigEndian.Uint16(ba[:2]))
	switch sm {
	case SMRequest:
	case SMSuccess:
	case SMFailure:
	case SMIndication:
		break
	default:
		return false
	}
	size := int(binary.BigEndian.Uint16(ba[2:4]))
	return (size+20) == len(ba) && binary.BigEndian.Uint32(ba[4:8]) == stunMagic
}
