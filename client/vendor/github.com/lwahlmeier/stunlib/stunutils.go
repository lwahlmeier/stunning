package stunlib // import "github.com/lwahlmeier/stunlib"

import "encoding/binary"

const (
	stunMagic            = 0x2112a442
	stunShortMagic       = 0x2112
	stunFingerPrintMagic = 0x5354554e
)

func UnmaskAddress(tid TransactionID, address []byte) []byte {
	na := make([]byte, len(address))
	na[0] = (byte)(address[0] ^ 0x21)
	na[1] = (byte)(address[1] ^ 0x12)
	na[2] = (byte)(address[2] ^ 0xa4)
	na[3] = (byte)(address[3] ^ 0x42)
	tidbb := tid.GetTID()
	for i := 4; i < len(na); i++ {
		na[i] = address[i] ^ tidbb[i-4]
	}
	return na
}

func MaskAddress(tid TransactionID, address []byte) []byte {
	return UnmaskAddress(tid, address)
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
