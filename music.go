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
	musicStreamer        beep.StreamSeekCloser
	musicFormat          beep.Format
	controlStreamer      *beep.Ctrl
	playing              bool
	paused               bool
	playMutex            sync.Mutex
	done                 chan bool
	playPauseButton      *widget.Button
	restartButton        *widget.Button
	progressBar          *widget.ProgressBar
	currentPositionLabel *widget.Label
	totalDurationLabel   *widget.Label

	defaultBufferLength = 100
	defaultSampleRate   = 44100
)

func main() {
	// Check if a file path is provided as a command-line argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go_music <file-path>")
		return
	}

	audioFilePath := os.Args[1]

	// Init soundcard
	var sampleRate beep.SampleRate = beep.SampleRate(defaultSampleRate)
	speaker.Init(sampleRate, sampleRate.N(time.Duration(defaultBufferLength)*time.Millisecond))
	go playAudio(audioFilePath)

	// Initialize Fyne app
	myApp := app.New()
	myWindow := myApp.NewWindow("Music Player")

	// Play/Pause button
	playPauseButton = widget.NewButton("Pause", func() {
		go togglePlayPause()
	})

	// Restart button
	restartButton = widget.NewButton("Restart", func() {
		go restartPlayback()
	})

	// Progress bar
	progressBar = widget.NewProgressBar()

	// Time labels
	currentPositionLabel = widget.NewLabel("Current: 00:00")
	totalDurationLabel = widget.NewLabel("Total: 00:00")

	// Add widgets to the window
	myWindow.SetContent(container.NewVBox(
		widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(audioFilePath))),
		playPauseButton,
		restartButton,
		progressBar,
		container.NewHBox(currentPositionLabel, totalDurationLabel),
	))

	myWindow.Resize(fyne.NewSize(400, 200))
	myWindow.Show()
	myApp.Run()
}

func togglePlayPause() {
	if playing || paused {
		speaker.Lock()
		if controlStreamer != nil {
			controlStreamer.Paused = !controlStreamer.Paused
			if controlStreamer.Paused {
				playPauseButton.SetText("Play")
			} else {
				playPauseButton.SetText("Pause")
			}
		}
		speaker.Unlock()
	}
}

func playAudio(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	ext := filepath.Ext(filePath)

	speaker.Clear()

	switch ext {
	case ".mp3":
		musicStreamer, musicFormat, err = mp3.Decode(file)
	case ".wav":
		musicStreamer, musicFormat, err = wav.Decode(file)
	case ".ogg":
		musicStreamer, musicFormat, err = vorbis.Decode(file)
	case ".flac":
		musicStreamer, musicFormat, err = flac.Decode(file)
	default:
		panic("Unsupported audio format: " + ext)
	}
	if err != nil {
		panic(err)
	}
	defer musicStreamer.Close()

	if musicFormat.SampleRate == 44100 {
		controlStreamer = &beep.Ctrl{Streamer: musicStreamer, Paused: false}
	} else {
		controlStreamer = &beep.Ctrl{Streamer: beep.Resample(4, musicFormat.SampleRate, 44100, musicStreamer), Paused: false}
	}

	done = make(chan bool)
	speaker.Play(beep.Seq(controlStreamer, beep.Callback(func() {
		done <- true
	})))

	playing = true
	paused = false
	go updateProgressBar()

	<-done

	playMutex.Lock()
	playing = false
	paused = false
	playPauseButton.SetText("Play")
	playMutex.Unlock()
}

func restartPlayback() {
	speaker.Lock()
	if musicStreamer != nil {

		if err := musicStreamer.Seek(0); err != nil {
			fmt.Println("Error restarting playback:", err)
			return
		}
	}
	speaker.Unlock()
	go updateProgressBar()
}

func formatTime(samples int, sampleRate beep.SampleRate) string {
	seconds := float64(samples) / float64(sampleRate)
	minutes := int(seconds) / 60
	remainingSeconds := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d", minutes, remainingSeconds)
}

func updateProgressBar() {
	for playing {
		time.Sleep(200 * time.Millisecond)
		speaker.Lock()
		if controlStreamer != nil {
			position := musicStreamer.Position()
			progressBar.SetValue(float64(position) / float64(musicStreamer.Len()))

			// Update time labels
			currentPosition := formatTime(position, musicFormat.SampleRate)
			totalDuration := formatTime(musicStreamer.Len(), musicFormat.SampleRate)
			currentPositionLabel.SetText("Current: " + currentPosition)
			totalDurationLabel.SetText("Total: " + totalDuration)
		} else {
			progressBar.SetValue(0)
			currentPositionLabel.SetText("Current: 00:00")
			totalDurationLabel.SetText("Total: 00:00")
		}
		speaker.Unlock()
	}

}
