package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

var (
	streamer    beep.StreamSeekCloser
	format      beep.Format
	playing     bool
	paused      bool
	playMutex   sync.Mutex
	done        chan bool
	playPauseButton *widget.Button
)

func main() {
	// Check if a file path is provided as a command-line argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go_music <file-path>")
		return
	}

	audioFilePath := os.Args[1]

	// Initialize Fyne app
	myApp := app.New()
	myWindow := myApp.NewWindow("Music Player")

	// Play/Pause button
	playPauseButton = widget.NewButton("Play", func() {
		go togglePlayPause(audioFilePath)
	})

	// Add button to the window
	myWindow.SetContent(container.NewVBox(
		widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(audioFilePath))),
		playPauseButton,
	))
	myWindow.Resize(fyne.NewSize(300, 150))
	myWindow.Show()
	myApp.Run()
}

func togglePlayPause(filePath string) {
	playMutex.Lock()
	defer playMutex.Unlock()

	if !playing {
		// Start playing
		playing = true
		paused = false
		playPauseButton.SetText("Pause")
		go playAudio(filePath)
	} else if paused {
		// Resume playback
		paused = false
		speaker.Unlock()
		playPauseButton.SetText("Pause")
	} else {
		// Pause playback
		paused = true
		speaker.Lock()
		playPauseButton.SetText("Play")
	}
}

func playAudio(filePath string) {
	// Open the audio file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode audio based on file extension
	ext := filepath.Ext(filePath)

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(file)
	case ".flac":
		streamer, format, err = flac.Decode(file)
	default:
		panic("Unsupported audio format: " + ext)
	}
	if err != nil {
		panic(err)
	}
	defer streamer.Close()

	// Initialize speaker
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	// Play the audio
	done = make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish or until paused
	select {
	case <-done:
	case <-time.After(100 * time.Hour): // Simulate a very long pause
	}

	playMutex.Lock()
	playing = false
	paused = false
	playPauseButton.SetText("Play")
	playMutex.Unlock()
}
