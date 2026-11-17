package cascade

import "unsafe"

// parseTLSSNI extracts the SNI hostname from the first TLS record if present.
// The returned string aliases record until record is no longer used.
func parseTLSSNI(record []byte) (string, bool) {
	if len(record) < 5 || record[0] != 0x16 {
		return "", false
	}
	payload := record[5:]
	if len(payload) < 4 || payload[0] != 0x01 {
		return "", false
	}
	helloLen := int(payload[1])<<16 | int(payload[2])<<8 | int(payload[3])
	bodyEnd := 4 + helloLen
	if helloLen < 34 || bodyEnd > len(payload) {
		return "", false
	}
	body := payload[4:bodyEnd]
	pos := 34 // legacy version + random
	if pos >= len(body) {
		return "", false
	}
	sessionLen := int(body[pos])
	pos++
	if pos+sessionLen+2 > len(body) {
		return "", false
	}
	pos += sessionLen
	cipherLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	if pos+cipherLen+1 > len(body) {
		return "", false
	}
	pos += cipherLen
	compLen := int(body[pos])
	pos++
	if pos+compLen+2 > len(body) {
		return "", false
	}
	pos += compLen
	extLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	end := pos + extLen
	if end > len(body) {
		return "", false
	}
	for pos+4 <= end {
		typ := int(body[pos])<<8 | int(body[pos+1])
		extSize := int(body[pos+2])<<8 | int(body[pos+3])
		pos += 4
		if pos+extSize > end {
			break
		}
		if typ == 0x0000 {
			data := body[pos : pos+extSize]
			if len(data) < 2 {
				break
			}
			listLen := int(data[0])<<8 | int(data[1])
			off := 2
			listEnd := off + listLen
			if listEnd > len(data) {
				break
			}
			for off+3 <= listEnd {
				nameType := data[off]
				nameLen := int(data[off+1])<<8 | int(data[off+2])
				off += 3
				if off+nameLen > listEnd {
					break
				}
				if nameType == 0 {
					if nameLen == 0 {
						break
					}
					return unsafe.String(&data[off], nameLen), true
				}
				off += nameLen
			}
		}
		pos += extSize
	}
	return "", false
}