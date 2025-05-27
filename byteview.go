package LCache_go

// readonly byte slice view for cache data

type ByteView struct {
	b []byte
}

func (b *ByteView) len() int {
	return len(b.b)
}
func (b *ByteView) byteSlice() []byte {
	return cloneBytes(b.b)
}

func cloneBytes(b []byte) []byte {
	rs := make([]byte, len(b))
	copy(rs, b)
	return rs
}
