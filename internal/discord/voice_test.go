package discord

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

func TestPCMEncoderProducesDecodableStereoOpus(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required for independent Opus decoding")
	}
	encoder, err := newPCMEncoder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var packets [][]byte
	for block := range 40 {
		frames := make([]mixer.Frame, opusSamples)
		for i := range frames {
			n := float64(block*opusSamples + i)
			frames[i] = mixer.Frame{
				Left:  0.5 * math.Sin(2*math.Pi*440*n/48000),
				Right: 0.5 * math.Sin(2*math.Pi*880*n/48000),
			}
		}
		data := mixer.EncodeFloat32(frames)
		for _, chunk := range [][]byte{data[:800], data[800:]} {
			if _, err := encoder.Write(chunk); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case packet := <-encoder.packets:
			packets = append(packets, packet)
		default:
			t.Fatal("missing encoded frame")
		}
	}
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "ogg", "-i", "pipe:0", "-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1")
	cmd.Stdin = bytes.NewReader(testOgg(packets))
	pcm, err := cmd.Output()
	if err != nil {
		t.Fatalf("independent Opus decoder rejected packets: %v", err)
	}
	if len(pcm) < 48000*2 {
		t.Fatalf("decoded audio too short: %d bytes", len(pcm))
	}
	for channel, frequency := range []float64{440, 880} {
		wanted := toneMagnitude(pcm, channel, frequency)
		other := toneMagnitude(pcm, channel, 1320-frequency)
		if wanted < 0.05 || wanted < other*3 {
			t.Fatalf("channel %d: wanted tone %.3f, other %.3f", channel, wanted, other)
		}
	}
}

func TestPCMEncoderSilenceToDualTone(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required for independent Opus decoding")
	}
	for _, silenceFrames := range []int{1, 50, 150} {
		t.Run(fmt.Sprint(silenceFrames), func(t *testing.T) {
			encoder, err := newPCMEncoder(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var packets [][]byte
			for block := range silenceFrames + 250 {
				frames := make([]mixer.Frame, opusSamples)
				for i := range frames {
					n := float64(block*opusSamples + i)
					var sample float64
					if block >= silenceFrames {
						sample = 0.12 * math.Sin(2*math.Pi*440*n/48000)
						if block >= silenceFrames+50 {
							sample += 0.12 * math.Sin(2*math.Pi*880*n/48000)
						}
					}
					frames[i] = mixer.Frame{Left: sample, Right: sample}
				}
				if _, err := encoder.Write(mixer.EncodeFloat32(frames)); err != nil {
					t.Fatal(err)
				}
				packets = append(packets, <-encoder.packets)
			}
			cmd := exec.Command("ffmpeg", "-v", "error", "-f", "ogg", "-i", "pipe:0", "-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1")
			cmd.Stdin = bytes.NewReader(testOgg(packets))
			pcm, err := cmd.Output()
			if err != nil {
				t.Fatalf("Opus decoder: %v", err)
			}
			checkDualTone(t, pcm, silenceFrames)
		})
	}
}

func checkDualTone(t *testing.T, pcm []byte, silenceFrames int) {
	t.Helper()
	for second := 1; second < 4; second++ {
		offset := (silenceFrames + 50 + second*50) * opusSamples * 4
		end := offset + 48000*4
		if end > len(pcm) {
			t.Fatal("decoded audio was truncated")
		}
		for _, freq := range []float64{440, 880} {
			amplitude := toneMagnitude(pcm[offset:end], 0, freq)
			if amplitude < 0.035 {
				t.Errorf("second %d frequency %.0f: magnitude %.4f", second, freq, amplitude)
			}
		}
	}
}

func toneMagnitude(pcm []byte, channel int, frequency float64) float64 {
	var sin, cos float64
	count := len(pcm) / 4
	for i := 960; i < count; i++ {
		sample := float64(int16(binary.LittleEndian.Uint16(pcm[4*i+2*channel:]))) / 32768
		phase := 2 * math.Pi * frequency * float64(i) / 48000
		sin += sample * math.Sin(phase)
		cos += sample * math.Cos(phase)
	}
	return math.Hypot(sin, cos) / float64(count-960)
}

func TestPCMEncoderRejectsInvalid(t *testing.T) {
	for _, sample := range []float32{float32(math.NaN()), float32(math.Inf(1)), 1.01, -1.01} {
		encoder, _ := newPCMEncoder(context.Background())
		if _, err := encoder.Write(mixer.EncodeFloat32([]mixer.Frame{{Left: float64(sample)}})); err == nil {
			t.Fatalf("accepted invalid sample %v", sample)
		}
	}
	encoder, _ := newPCMEncoder(context.Background())
	if _, err := encoder.Write([]byte{1}); err == nil {
		t.Fatal("accepted incomplete stereo frame")
	}
}

func TestPCMEncoderBackpressurePreservesPackets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	encoder, err := newPCMEncoder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := newPCMEncoder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var expected [][]byte
	var blocks [][]byte
	for block := range packetQueue + 1 {
		frames := make([]mixer.Frame, opusSamples)
		for i := range frames {
			sample := 0.2 * math.Sin(2*math.Pi*440*float64(block*opusSamples+i)/48000)
			frames[i] = mixer.Frame{Left: sample, Right: sample}
		}
		data := mixer.EncodeFloat32(frames)
		blocks = append(blocks, data)
		if _, err := reference.Write(data); err != nil {
			t.Fatal(err)
		}
		expected = append(expected, <-reference.packets)
	}
	for _, block := range blocks[:packetQueue] {
		if _, err := encoder.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() { _, err := encoder.Write(blocks[packetQueue]); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("full queue did not apply backpressure: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	actual := [][]byte{<-encoder.packets}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("encoder did not resume after backpressure")
	}
	for range packetQueue {
		actual = append(actual, <-encoder.packets)
	}
	for i := range actual {
		if !bytes.Equal(actual[i], expected[i]) {
			t.Fatalf("packet %d was dropped or reordered", i)
		}
	}
}

func TestPCMEncoderBackpressureUnblocksOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	encoder, err := newPCMEncoder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data := mixer.EncodeFloat32(make([]mixer.Frame, opusSamples))
	for range packetQueue {
		if _, err := encoder.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() { _, err := encoder.Write(data); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled encoder, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("encoder did not stop after cancellation")
	}
}

type fakeVoiceOutput struct {
	ready    atomic.Bool
	checked  chan struct{}
	speaking chan struct{}
	writes   chan []byte
	writeErr error
}

func (f *fakeVoiceOutput) Ready() bool {
	ready := f.ready.Load()
	f.checked <- struct{}{}
	return ready
}
func (f *fakeVoiceOutput) Speaking(context.Context) error {
	f.speaking <- struct{}{}
	return nil
}
func (f *fakeVoiceOutput) Write(packet []byte) error {
	f.writes <- packet
	return f.writeErr
}

func TestVoiceSenderDiscardsHeldFramesAndStopsOnFailure(t *testing.T) {
	output := &fakeVoiceOutput{checked: make(chan struct{}, 2), speaking: make(chan struct{}, 1), writes: make(chan []byte, 2)}
	packets := make(chan []byte, 2)
	failures := make(chan error, 1)
	result := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { result <- sendPackets(ctx, output, packets, failures, nil, time.Now) }()
	packets <- []byte{1}
	<-output.checked
	output.ready.Store(true)
	packets <- []byte{2}
	select {
	case <-output.speaking:
	case <-time.After(time.Second):
		t.Fatal("sender did not signal speaking before audio")
	}
	select {
	case packet := <-output.writes:
		if !bytes.Equal(packet, []byte{2}) {
			t.Fatalf("sent held packet: %v", packet)
		}
	case <-time.After(time.Second):
		t.Fatal("ready sender did not send")
	}
	failures <- errors.New("connection lost")
	select {
	case err := <-result:
		if err == nil || err.Error() != "connection lost" {
			t.Fatalf("lost connection error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("sender did not stop after connection failure")
	}
}

func TestVoiceStartupRejectsInvalidIDsAndDoesNotExposeToken(t *testing.T) {
	for _, cfg := range []Config{
		{Token: "secret-token", GuildID: "18446744073709551616", ChannelID: "1"},
		{Token: "secret-token", GuildID: "1", ChannelID: "2"},
	} {
		_, err := startVoice(context.Background(), cfg)
		if err == nil || strings.Contains(err.Error(), cfg.Token) {
			t.Fatalf("invalid setup returned unsafe error: %v", err)
		}
	}
}

func TestVoiceSenderStopsOnWriteFailureAndCancellation(t *testing.T) {
	for _, failure := range []bool{false, true} {
		output := &fakeVoiceOutput{checked: make(chan struct{}, 1), speaking: make(chan struct{}, 1), writes: make(chan []byte, 1)}
		output.ready.Store(true)
		packets := make(chan []byte, 1)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		if failure {
			output.writeErr = errors.New("unsafe underlying detail")
		}
		go func() { result <- sendPackets(ctx, output, packets, nil, nil, time.Now) }()
		if failure {
			packets <- []byte{1}
		} else {
			cancel()
		}
		select {
		case err := <-result:
			if failure && (err == nil || err.Error() != "Discord voice audio send failed") {
				t.Fatalf("write failure not sanitized: %v", err)
			}
			if !failure && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("sender did not stop")
		}
		cancel()
	}
}

func TestVoiceSenderReannouncesSpeakingForLateJoiners(t *testing.T) {
	output := &fakeVoiceOutput{checked: make(chan struct{}, 4), speaking: make(chan struct{}, 4), writes: make(chan []byte, 4)}
	output.ready.Store(true)
	packets := make(chan []byte, 1)
	refresh := make(chan struct{})
	clock := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- sendPackets(ctx, output, packets, nil, refresh, func() time.Time { return <-clock }) }()
	defer func() { cancel(); <-result }()
	start := time.Unix(100, 0)
	send := func(at time.Time, wantSpeaking bool) {
		t.Helper()
		clock <- at
		packets <- []byte{1}
		select {
		case <-output.writes:
		case <-time.After(time.Second):
			t.Fatal("sender did not write")
		}
		wait := 50 * time.Millisecond
		if wantSpeaking {
			wait = time.Second
		}
		select {
		case <-output.speaking:
			if !wantSpeaking {
				t.Fatal("unexpected speaking event")
			}
		case <-time.After(wait):
			if wantSpeaking {
				t.Fatal("missing speaking event")
			}
		}
	}
	send(start, true)
	send(start.Add(time.Second), false)
	refresh <- struct{}{}
	send(start.Add(2*time.Second), true)
	send(start.Add(8*time.Second), true)
}

type testDaveCallbacks struct{}

func (testDaveCallbacks) SendMLSKeyPackage([]byte) error        { return nil }
func (testDaveCallbacks) SendMLSCommitWelcome([]byte) error     { return nil }
func (testDaveCallbacks) SendReadyForTransition(uint16) error   { return nil }
func (testDaveCallbacks) SendInvalidCommitWelcome(uint16) error { return nil }

type testDaveConn struct {
	voice.Conn
	session godave.Session
	udp     voice.UDPConn
}

func (c testDaveConn) DAVE() godave.Session { return c.session }
func (c testDaveConn) UDP() voice.UDPConn   { return c.udp }

func TestDaveReadiness(t *testing.T) {
	session := newDaveSession(slog.New(slog.DiscardHandler), "123456789", testDaveCallbacks{})
	defer session.Close()
	session.SetChannelID(987654321)
	output := discordOutput{testDaveConn{session: session}}
	session.OnSelectProtocolAck(1)
	if output.Ready() {
		t.Fatal("DAVE negotiation must hold audio until ready")
	}
	session.OnSelectProtocolAck(0)
	if !output.Ready() {
		t.Fatal("transport-only session should not wait for an epoch")
	}
	session.OnSelectProtocolAck(1)
	if output.Ready() {
		t.Fatal("switching from transport-only to DAVE must hold audio until ready")
	}
}

type testDaveSession struct{ godave.Session }

func (testDaveSession) Ready() bool                            { return false }
func (testDaveSession) OnSelectProtocolAck(uint16)             {}
func (testDaveSession) OnDavePrepareEpoch(int, uint16)         {}
func (testDaveSession) OnDavePrepareTransition(uint16, uint16) {}
func (testDaveSession) OnDaveExecuteTransition(uint16)         {}

func TestDaveReadinessAcrossTransitions(t *testing.T) {
	session := &daveSession{Session: testDaveSession{}, transitions: make(map[uint16]uint16)}
	session.OnSelectProtocolAck(0)
	session.OnDavePrepareTransition(2, 1)
	if !session.Ready() {
		t.Fatal("DAVE must remain disabled until the transition executes")
	}
	session.OnDaveExecuteTransition(2)
	if session.Ready() {
		t.Fatal("DAVE must hold audio until the encrypted session is ready")
	}
	session.OnDavePrepareTransition(3, 0)
	session.OnDaveExecuteTransition(3)
	if !session.Ready() {
		t.Fatal("disabled DAVE must permit audio after its transition")
	}
	session.OnDavePrepareEpoch(1, 1)
	if session.Ready() {
		t.Fatal("a new DAVE epoch must hold audio during negotiation")
	}
}

func TestDisgoUDPSendsOpus(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := server.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	datagrams := make(chan []byte, 2)
	serverErrors := make(chan error, 1)
	go func() {
		buf := make([]byte, 2048)
		n, peer, err := server.ReadFromUDP(buf)
		if err != nil {
			serverErrors <- err
			return
		}
		if n != 74 {
			serverErrors <- errors.New("invalid discovery request")
			return
		}
		response := make([]byte, 74)
		binary.BigEndian.PutUint16(response, 2)
		binary.BigEndian.PutUint16(response[2:], 70)
		copy(response[4:8], buf[4:8])
		copy(response[8:], "127.0.0.1")
		binary.BigEndian.PutUint16(response[72:], uint16(peer.Port))
		if _, err := server.WriteToUDP(response, peer); err != nil {
			serverErrors <- err
			return
		}
		for range 2 {
			n, _, err := server.ReadFromUDP(buf)
			if err != nil {
				serverErrors <- err
				return
			}
			datagrams <- bytes.Clone(buf[:n])
		}
	}()
	session := newDaveSession(slog.New(slog.DiscardHandler), "123456789", testDaveCallbacks{})
	defer session.Close()
	session.SetChannelID(987654321)
	session.OnSelectProtocolAck(0)
	udp := voice.NewUDPConn(session, nil, voice.WithUDPConnLogger(slog.New(slog.DiscardHandler)))
	defer udp.Close()
	addr := server.LocalAddr().(*net.UDPAddr)
	if _, _, err := udp.Open(context.Background(), "127.0.0.1", addr.Port, 42); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	if err := udp.SetSecretKey(voice.EncryptionModeAEADAES256GCMRTPSize, key); err != nil {
		t.Fatal(err)
	}
	encoder, err := newPCMEncoder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Write(mixer.EncodeFloat32(make([]mixer.Frame, opusSamples))); err != nil {
		t.Fatal(err)
	}
	packet := <-encoder.packets
	output := discordOutput{testDaveConn{session: session, udp: udp}}
	for range 2 {
		if err := output.Write(packet); err != nil {
			t.Fatal(err)
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		select {
		case data := <-datagrams:
			if len(data) < 32 || data[0] != 0x80 || data[1] != voice.RTPPayloadType ||
				binary.BigEndian.Uint16(data[2:]) != uint16(i) ||
				binary.BigEndian.Uint32(data[4:]) != uint32(i*opusSamples) ||
				binary.BigEndian.Uint32(data[8:]) != 42 {
				t.Fatal("invalid outgoing RTP header")
			}
			nonce := make([]byte, aead.NonceSize())
			copy(nonce, data[len(data)-4:])
			payload, err := aead.Open(nil, nonce, data[12:len(data)-4], data[:12])
			if err != nil || !bytes.Equal(payload, packet) {
				t.Fatalf("outgoing RTP did not contain the encoded Opus packet: %v", err)
			}
		case err := <-serverErrors:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("no RTP datagram received")
		}
	}
}

func testOgg(packets [][]byte) []byte {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8], head[9] = 1, 2
	binary.LittleEndian.PutUint32(head[12:], 48000)
	tags := make([]byte, 16)
	copy(tags, "OpusTags")
	var stream bytes.Buffer
	stream.Write(testOggPage(head, 2, 0, 0))
	stream.Write(testOggPage(tags, 0, 0, 1))
	for i, packet := range packets {
		flags := byte(0)
		if i == len(packets)-1 {
			flags = 4
		}
		stream.Write(testOggPage(packet, flags, uint64(i+1)*opusSamples, uint32(i+2)))
	}
	return stream.Bytes()
}

func testOggPage(packet []byte, flags byte, granule uint64, sequence uint32) []byte {
	lacing := bytes.Repeat([]byte{255}, len(packet)/255)
	lacing = append(lacing, byte(len(packet)%255))
	page := make([]byte, 27, 27+len(lacing)+len(packet))
	copy(page, "OggS")
	page[5], page[26] = flags, byte(len(lacing))
	binary.LittleEndian.PutUint64(page[6:], granule)
	binary.LittleEndian.PutUint32(page[14:], 1)
	binary.LittleEndian.PutUint32(page[18:], sequence)
	page = append(append(page, lacing...), packet...)
	var crc uint32
	for _, b := range page {
		crc ^= uint32(b) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	binary.LittleEndian.PutUint32(page[22:], crc)
	return page
}
