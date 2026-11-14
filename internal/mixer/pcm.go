package mixer

import (
	"encoding/binary"
	"math"
)

func EncodeFloat32(frames []Frame) []byte {
	data := make([]byte, len(frames)*8)
	for i, frame := range frames {
		binary.LittleEndian.PutUint32(data[i*8:], math.Float32bits(float32(frame.Left)))
		binary.LittleEndian.PutUint32(data[i*8+4:], math.Float32bits(float32(frame.Right)))
	}
	return data
}
