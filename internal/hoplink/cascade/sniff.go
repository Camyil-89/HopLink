package cascade

// parseTLSSNI extracts the SNI hostname from the first TLS record if present.
func parseTLSSNI(record []byte) (string, bool) {
	if len(record) < 5 || record[0] != 0x16 {
		return "", false
	}
	// TLS record length at bytes 3-4; skip record header.
	payload := record[5:]
	if len(payload) < 4 || payload[0] != 0x01 {
		return "", false
	}
	pos := 4 + int(payload[1])<<16 + int(payload[2])<<8 + int(payload[3])
	if pos > len(payload) {
		return "", false
	}
	if pos+2 > len(payload) {
		return "", false
	}
	sessionLen := int(payload[pos])<<8 + int(payload[pos+1])
	pos += 2 + sessionLen
	if pos+2 > len(payload) {
		return "", false
	}
	cipherLen := int(payload[pos])<<8 + int(payload[pos+1])
	pos += 2 + cipherLen
	if pos+1 > len(payload) {
		return "", false
	}
	compLen := int(payload[pos])
	pos += 1 + compLen
	if pos+2 > len(payload) {
		return "", false
	}
	extLen := int(payload[pos])<<8 + int(payload[pos+1])
	pos += 2
	end := pos + extLen
	if end > len(payload) {
		return "", false
	}
	for pos+4 <= end {
		typ := int(payload[pos])<<8 + int(payload[pos+1])
		l := int(payload[pos+2])<<8 + int(payload[pos+3])
		pos += 4
		if pos+l > end {
			break
		}
		if typ == 0x0000 { // server_name
			data := payload[pos : pos+l]
			if len(data) < 2 {
				break
			}
			listLen := int(data[0])<<8 + int(data[1])
			if listLen+2 > len(data) {
				break
			}
			off := 2
			for off+3 <= 2+listLen {
				nameType := data[off]
				nameLen := int(data[off+1])<<8 + int(data[off+2])
				off += 3
				if off+nameLen > len(data) {
					break
				}
				if nameType == 0 {
					host := string(data[off : off+nameLen])
					return host, host != ""
				}
				off += nameLen
			}
		}
		pos += l
	}
	return "", false
}