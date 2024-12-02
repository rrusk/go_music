package main

import (
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"os"
	"time"
)

func main() {
	// Initialize Fyne app
	myApp := app.New()
	myWindow := myApp.NewWindow("Music Player")

	// Play button
	playButton := widget.NewButton("Play", func() {
		go playAudio("example.mp3") // Run audio playback in a separate goroutine
	})

	// Add play button to window
	myWindow.SetContent(container.NewVBox(playButton))
    myWindow.Resize(fyne.NewSize(300, 150))
	myWindow.ShowAndRun()
}

func playAudio(filePath string) {
	// Open the MP3 file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode MP3 and initialize speaker
	streamer, format, err := mp3.Decode(file)
	if err != nil {
		panic(err)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	// Play the audio in a blocking manner
	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish
	<-done
}
