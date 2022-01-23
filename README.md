# Voicy
A simple way to play music on Discord using Golang

## 📚 Requirements
- Voicy only supports [arikawa](https://github.com/diamondburned/arikawa) for now.
- You need to have [ffmpeg](https://www.ffmpeg.org/download.html) installed on your system.

## 📋 Example
```go
package main

import (
	"context"

	"github.com/ItsClairton/voicy"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/kkdai/youtube/v2"
)

var (
	token   = "Your bot token here"
	channel = discord.ChannelID(806346482134417433) // Voice channel ID here
	client  = youtube.Client{}
)

func main() {
	session := state.NewWithIntents("Bot "+token, gateway.IntentGuilds, gateway.IntentGuildVoiceStates)
	if err := session.Open(context.Background()); err != nil {
		panic(err)
	}

	defer session.Close()

	voice, err := voicy.New(context.Background(), session, channel)
	if err != nil {
		panic(err)
	}

	video, err := client.GetVideo("https://www.youtube.com/watch?v=LNuVDtUUmd4")
	if err != nil {
		panic(err)
	}

	stream, err := client.GetStreamURL(video, &video.Formats.WithAudioChannels()[0])
	if err != nil {
		panic(err)
	}

	if err := voice.PlayURL(stream, false); err != nil {
		panic(err)
	}
}
```

## ❓ Questions or bugs?
Don't hesitate to create an issue or a PR.