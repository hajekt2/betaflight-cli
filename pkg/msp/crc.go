package msp

// CRC8DVBUpdate advances the DVB-S2 CRC8 accumulator over data, allowing
// checksums over frame parts without concatenating them into one buffer.
func CRC8DVBUpdate(crc byte, data []byte) byte {
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

func CRC8DVB(data []byte) byte {
	return CRC8DVBUpdate(0, data)
}
