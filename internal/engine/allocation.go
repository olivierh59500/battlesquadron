package engine

// OriginalFlyingArmor applies the original allocator's byte-sized arithmetic.
// A one-player game removes one quarter after incrementing the byte; $FF+1
// wraps to zero before the shift, preserving the original $FF sentinel.
func OriginalFlyingArmor(health int, twoPlayers bool) int {
	armor := byte(health)
	if !twoPlayers {
		armor -= (armor + 1) >> 2
	}
	return int(armor)
}
