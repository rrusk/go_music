package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	playingSong          bool
	done                 chan bool
	playPauseButton      *widget.Button
	restartButton        *widget.Button
	nextButton           *widget.Button
	prevButton           *widget.Button
	progressBar          *widget.ProgressBar
	currentPositionLabel *widget.Label
	totalDurationLabel   *widget.Label
	currentSongLabel     *widget.Label

	playlist         []string
	currentSongIndex int

	defaultBufferLength = 100
	defaultSampleRate   = 44100
)

func main() {
	// Define the directory containing the songs
	directory := "/home/rrusk/Music/MUSICCOMP"

	// Initialize the playlist
	playlist = []string{}
	if err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			switch filepath.Ext(path) {
			case ".mp3", ".wav", ".ogg", ".flac":
				playlist = append(playlist, path)
			}
		}
		return nil
	}); err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	if len(playlist) == 0 {
		fmt.Println("No supported audio files found in directory")
		return
	}

	// Init soundcard
	var sampleRate beep.SampleRate = beep.SampleRate(defaultSampleRate)
	speaker.Init(sampleRate, sampleRate.N(time.Duration(defaultBufferLength)*time.Millisecond))

	// Initialize Fyne app
	myApp := app.New()
	myWindow := myApp.NewWindow("Music Player")

	// UI Elements
	playPauseButton = widget.NewButton("Pause", func() {
		go togglePlayPause()
	})

	restartButton = widget.NewButton("Restart", func() {
		go restartPlayback()
	})

	nextButton = widget.NewButton("Next", func() {
		go playNextSong()
	})

	prevButton = widget.NewButton("Previous", func() {
		go playPreviousSong()
	})

	progressBar = widget.NewProgressBar()

	currentPositionLabel = widget.NewLabel("Current: 00:00")
	totalDurationLabel = widget.NewLabel("Total: 00:00")
	currentSongLabel = widget.NewLabel("Now Playing: " + filepath.Base(playlist[0]))

	// Layout
	myWindow.SetContent(container.NewVBox(
		currentSongLabel,
		playPauseButton,
		restartButton,
		container.NewHBox(prevButton, nextButton),
		progressBar,
		container.NewHBox(currentPositionLabel, totalDurationLabel),
	))

	myWindow.Resize(fyne.NewSize(400, 200))
	myWindow.Show()

	// Start playing the first song
	currentSongIndex = 0
	go playAudio(playlist[currentSongIndex])
	myApp.Run()
}

func togglePlayPause() {
	if playingSong {
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

	if musicFormat.SampleRate == beep.SampleRate(defaultSampleRate) {
		controlStreamer = &beep.Ctrl{Streamer: musicStreamer, Paused: false}
	} else {
		controlStreamer = &beep.Ctrl{Streamer: beep.Resample(4, musicFormat.SampleRate, beep.SampleRate(defaultSampleRate), musicStreamer), Paused: false}
	}

	done = make(chan bool)
	speaker.Play(beep.Seq(controlStreamer, beep.Callback(func() {
		done <- true
	})))

	playingSong = true
	currentSongLabel.SetText("Now Playing: " + filepath.Base(filePath))
	go updateProgressBar()

	<-done

	playingSong = false
	playPauseButton.SetText("Play")
	playNextSong() // Automatically play the next song when current one ends
}

func playNextSong() {
	if currentSongIndex < len(playlist)-1 {
		currentSongIndex++
		go playAudio(playlist[currentSongIndex])
	}
}

func playPreviousSong() {
	if currentSongIndex > 0 {
		currentSongIndex--
		go playAudio(playlist[currentSongIndex])
	}
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
	for playingSong {
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
