package engine

// DecodeBCDScore converts the original four packed decimal digits into the
// host's integer score. Motorola ABCD operates on these digits directly;
// interpreting $5000 as a binary word would incorrectly award 20,480 points.
func DecodeBCDScore(value uint16) int {
	return int(value>>12&15)*1000 + int(value>>8&15)*100 + int(value>>4&15)*10 + int(value&15)
}
