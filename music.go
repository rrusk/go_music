package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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

	// Play button
	playButton := widget.NewButton("Play", func() {
		go playAudio(audioFilePath) // Play the file provided via the command line
	})

	// Add play button to window
	myWindow.SetContent(container.NewVBox(
		widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(audioFilePath))),
		playButton,
	))
	myWindow.Resize(fyne.NewSize(300, 150))
	myWindow.ShowAndRun()
}

func playAudio(filePath string) {
	// Open the audio file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode audio based on file extension
	var streamer beep.StreamSeekCloser
	var format beep.Format
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

	// Play the audio in a blocking manner
	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish
	<-done
}
