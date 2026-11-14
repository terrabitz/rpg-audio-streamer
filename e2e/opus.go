package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
)

const opusFrameSamples = 960

// decodeOpus decodes 20 ms stereo Opus packets to 48 kHz s16le PCM. FFmpeg
// needs a container, so the packets are wrapped in a minimal Ogg Opus stream.
func decodeOpus(ctx context.Context, packets [][]byte) ([]byte, error) {
	if len(packets) == 0 {
		return nil, errors.New("no Opus packets to decode")
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "ogg", "-i", "pipe:0",
		"-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1")
	cmd.Stdin = bytes.NewReader(oggOpus(packets))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pcm, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("decode Opus: %w: %s", err, stderr.Bytes())
	}
	return pcm, nil
}

func oggOpus(packets [][]byte) []byte {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8] = 1
	head[9] = 2
	binary.LittleEndian.PutUint32(head[12:], sampleRate)
	tags := make([]byte, 16)
	copy(tags, "OpusTags")

	var stream bytes.Buffer
	stream.Write(oggPage(head, 0x02, 0, 0))
	stream.Write(oggPage(tags, 0, 0, 1))
	for i, packet := range packets {
		flags := byte(0)
		if i == len(packets)-1 {
			flags = 0x04
		}
		stream.Write(oggPage(packet, flags, uint64(i+1)*opusFrameSamples, uint32(i+2)))
	}
	return stream.Bytes()
}

func oggPage(packet []byte, flags byte, granule uint64, sequence uint32) []byte {
	lacing := bytes.Repeat([]byte{255}, len(packet)/255)
	lacing = append(lacing, byte(len(packet)%255))
	page := make([]byte, 27, 27+len(lacing)+len(packet))
	copy(page, "OggS")
	page[5] = flags
	binary.LittleEndian.PutUint64(page[6:], granule)
	binary.LittleEndian.PutUint32(page[14:], 1)
	binary.LittleEndian.PutUint32(page[18:], sequence)
	page[26] = byte(len(lacing))
	page = append(append(page, lacing...), packet...)
	binary.LittleEndian.PutUint32(page[22:], oggChecksum(page))
	return page
}

var oggCRCTable = func() (table [256]uint32) {
	for i := range table {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}()

func oggChecksum(page []byte) uint32 {
	var crc uint32
	for _, b := range page {
		crc = crc<<8 ^ oggCRCTable[byte(crc>>24)^b]
	}
	return crc
}
