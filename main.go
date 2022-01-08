package voicy

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
	"github.com/diamondburned/oggreader"
	"github.com/pkg/errors"
)

const (
	DestroyedState = iota
	StoppedState
	ChangingState
	PausedState
	PlayingState
)

var (
	ErrDestroyed      = errors.New("this session has been destroyed and can no longer be used")
	ErrAlreadyPlaying = errors.New("something is already playing")
)

type Session struct {
	conn *voice.Session

	source   string
	isOpus   bool
	position time.Duration

	state   int
	channel chan int

	mainCtx context.Context

	context context.Context
	cancel  context.CancelFunc
}

func New(context context.Context, state *state.State, ChannelID discord.ChannelID) (*Session, error) {
	conn, err := voice.NewSession(state)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create a voice session")
	}

	if err := conn.JoinChannel(context, ChannelID, false, true); err != nil {
		return nil, errors.Wrap(err, "unable to connect to voice channel")
	}

	return &Session{conn: conn, mainCtx: context, state: StoppedState}, nil
}

func (s *Session) PlayURL(source string, isOpus bool) error {
	if s.state == DestroyedState {
		return ErrDestroyed
	}

	if s.state > ChangingState {
		return ErrAlreadyPlaying
	}

	s.context, s.cancel = context.WithCancel(s.mainCtx)
	s.source, s.isOpus = source, isOpus
	defer s.stop()

	encoder := "copy"
	if !isOpus {
		encoder = "libopus"
	}

	ffmpeg := exec.CommandContext(s.context, "ffmpeg",
		"-loglevel", "error", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5", "-ss", strconv.Itoa(int(s.position.Seconds())),
		"-i", source, "-vn", "-codec", encoder, "-vbr", "off", "-frame_duration", "20", "-f", "opus", "-")

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get ffmpeg stdout")
	}

	var stderr bytes.Buffer
	ffmpeg.Stderr = &stderr

	if err := ffmpeg.Start(); err != nil {
		return errors.Wrap(err, "failed to start ffmpeg process")
	}

	if err := s.conn.Speaking(s.context, voicegateway.Microphone); err != nil {
		return errors.Wrap(err, "failed to send speaking packet to discord")
	}

	s.setState(PlayingState)

	if err := oggreader.DecodeBuffered(internalWriter{s}, stdout); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	if err, std := ffmpeg.Wait(), stderr.String(); err != nil && s.state != ChangingState && std != "" {
		return errors.Wrap(errors.New(std), "ffmpeg returned error")
	}

	if s.state == ChangingState {
		return s.PlayURL(s.source, s.isOpus)
	}

	return nil
}

func (s *Session) Seek(pos time.Duration) {
	if s.state < PausedState {
		return
	}

	s.position = pos
	s.setState(ChangingState)
	s.Stop()
}

func (s *Session) Pause() {
	if s.state != PlayingState {
		return
	}

	s.setState(PausedState)
	s.conn.Speaking(s.context, voicegateway.NotSpeaking)
}

func (s *Session) Resume() {
	if s.state != PausedState {
		return
	}

	s.conn.Speaking(s.context, voicegateway.Microphone)
	s.setState(PlayingState)
}

func (s *Session) State() int {
	return s.state
}

func (s *Session) PlaybackPosition() time.Duration {
	return s.position
}

func (s *Session) Stop() {
	if s.state < ChangingState {
		return
	}

	if s.state == PausedState {
		s.setState(PausedState)
	}

	s.cancel()
	s.waitAnyState()
}

func (s *Session) Destroy() {
	s.Stop()
	s.conn.Leave(s.mainCtx)
	s.setState(DestroyedState)
}

func (s *Session) setState(state int) {
	s.state = state

	if s.channel != nil {
		s.channel <- state
	}
}

func (s *Session) waitAnyState() int {
	if s.channel == nil {
		s.channel = make(chan int)
		defer func() {
			close(s.channel)
			s.channel = nil
		}()
	}

	return <-s.channel
}

func (s *Session) stop() {
	if s.state < ChangingState {
		return
	}

	s.cancel()
	s.position = 0
	s.conn.Speaking(s.mainCtx, voicegateway.NotSpeaking)
	s.setState(StoppedState)
}
