package msp

func CRC8DVB(data []byte) byte {
	var crc byte
	for _, b := range data {
		crc ^= b
		for range 8 {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0xd5
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
