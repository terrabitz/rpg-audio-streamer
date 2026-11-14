package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"os/exec"
	"testing"
)

func TestDecodeOpusRecoversTones(t *testing.T) {
	packets := encodeOpus(t, "aevalsrc=0.2*sin(440*2*PI*t)+0.3*sin(880*2*PI*t)|0.2*sin(440*2*PI*t)+0.3*sin(880*2*PI*t):s=48000:d=1.2")
	pcm, err := decodeOpus(context.Background(), packets)
	if err != nil {
		t.Fatal(err)
	}
	if frames := len(pcm) / 4; frames < len(packets)*960-960 || frames > len(packets)*960 {
		t.Fatalf("decoded %d frames from %d packets", frames, len(packets))
	}
	signal := analyze(pcm[len(pcm)-sampleRate*4:])
	if signal.Low < 0.17 || signal.High < 0.27 {
		t.Fatalf("decoded tones analyzed as %+v", signal)
	}
}

func TestDecodeOpusRejectsEmptyStream(t *testing.T) {
	if _, err := decodeOpus(context.Background(), nil); err == nil {
		t.Fatal("decoded an empty stream")
	}
}

func encodeOpus(t *testing.T, source string) [][]byte {
	t.Helper()
	output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", source,
		"-c:a", "libopus", "-frame_duration", "20", "-f", "ogg", "pipe:1").Output()
	if err != nil {
		t.Fatalf("encode Opus: %v", err)
	}
	var packets [][]byte
	var packet []byte
	for page := output; len(page) > 0; {
		if !bytes.HasPrefix(page, []byte("OggS")) || len(page) < 27 {
			t.Fatal("ffmpeg produced an invalid Ogg page")
		}
		segments := page[27 : 27+int(page[26])]
		body := page[27+len(segments):]
		for _, size := range segments {
			packet = append(packet, body[:size]...)
			body = body[size:]
			if size < 255 {
				packets = append(packets, packet)
				packet = nil
			}
		}
		page = body
	}
	if len(packets) < 3 || binary.LittleEndian.Uint64(packets[0][:8]) != binary.LittleEndian.Uint64([]byte("OpusHead")) {
		t.Fatal("ffmpeg did not produce an Opus stream")
	}
	return packets[2:]
}
