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
	streamer         beep.StreamSeekCloser
	format           beep.Format
	playing          bool
	paused           bool
	playMutex        sync.Mutex
	done             chan bool
	playPauseButton  *widget.Button
	restartButton    *widget.Button
	speakerLocked    bool // Tracks if the speaker is locked
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

	// Restart button
	restartButton = widget.NewButton("Restart", func() {
		go restartPlayback()
	})

	// Add buttons to the window
	myWindow.SetContent(container.NewVBox(
		widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(audioFilePath))),
		playPauseButton,
		restartButton,
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
		if speakerLocked {
			speaker.Unlock()
			speakerLocked = false
		}
		playPauseButton.SetText("Pause")
	} else {
		// Pause playback
		paused = true
		speaker.Lock()
		speakerLocked = true
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

	// Wait for playback to finish or until restart
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

func restartPlayback() {
	playMutex.Lock()
	defer playMutex.Unlock()

	if playing || paused {
		// Reset streamer position to the beginning
		if err := streamer.Seek(0); err != nil {
			fmt.Println("Error restarting playback:", err)
			return
		}

		if paused {
			// If paused, unlock the speaker to allow playback to resume later
			if speakerLocked {
				speaker.Unlock()
				speakerLocked = false
			}
		}

		// If playing, restart playback from the beginning
		if playing {
			playPauseButton.SetText("Pause")
		}
	}
}
