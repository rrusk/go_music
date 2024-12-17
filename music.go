package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/go-ini/ini"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
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
	volumeControl        *effects.Volume
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
	volumeSlider         *widget.Slider

	playlist         []string
	currentSongIndex int
	playlistList     *widget.List

	defaultBufferLength = 100
	defaultSampleRate   = 44100
	musicDir            string
	volume              float64
)

func main() {
	// Load configuration
	if err := loadConfiguration(); err != nil {
		fmt.Println("Failed to load configuration:", err)
		return
	}

	// Initialize playlist
	var err error
	playlist, err = initializePlaylist(musicDir)
	if err != nil {
		fmt.Println("Failed to initialize playlist:", err)
		return
	}

	if len(playlist) == 0 {
		fmt.Println("No supported audio files found in directory:", musicDir)
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

	currentPositionLabel = widget.NewLabel("00:00")
	totalDurationLabel = widget.NewLabel("00:00")
	currentSongLabel = widget.NewLabel(filepath.Base(playlist[0]))

	// Volume Slider
	volumeSlider = widget.NewSlider(0, 120)
	volumeSlider.SetValue(volume)
	volumeSlider.Step = 4
	volumeSlider.OnChanged = func(value float64) {
		adjustVolume(value)
	}

	// Playlist List
	playlistList = widget.NewList(
		func() int {
			return len(playlist)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			label.SetText(filepath.Base(playlist[id]))

			// Highlight the current song
			if id == currentSongIndex {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{Bold: false}
			}
		},
	)

	playlistList.OnSelected = func(id widget.ListItemID) {
		currentSongIndex = id
		go playAudio(playlist[currentSongIndex])
	}

	// Layout Adjustments
	playlistContainer := container.NewStack(container.NewVScroll(playlistList))
	playlistContainer.Resize(fyne.NewSize(600, 800))

	controlsContainer := container.New(layout.NewVBoxLayout(),
		currentSongLabel,
		container.NewHBox(currentPositionLabel, layout.NewSpacer(), totalDurationLabel),
		progressBar,
		container.NewHBox(
			container.NewVBox(widget.NewLabel("Volume"), volumeSlider),
			prevButton, playPauseButton, nextButton, restartButton),
	)

	mainContent := container.NewVBox(
		playlistContainer,
		controlsContainer,
	)

	myWindow.SetContent(mainContent)
	myWindow.Resize(fyne.NewSize(600, 800))
	myWindow.Show()

	// Start playing the first song
	currentSongIndex = 0
	go playAudio(playlist[currentSongIndex])
	myApp.Run()
}

func loadConfiguration() error {
	const configPath = "config.ini"

	// Default configuration values
	defaultVolume := 120.0
	defaultMusicDir := filepath.Join(os.Getenv("HOME"), "Music")

	// Check if config.ini exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default configuration
		cfg := ini.Empty()
		cfg.Section("user").Key("volume").SetValue(fmt.Sprintf("%.1f", defaultVolume))
		cfg.Section("user").Key("music_dir").SetValue(defaultMusicDir)
		cfg.Section("user").Key("song_max_playtime").SetValue("210")
		cfg.Section("user").Key("practice_type").SetValue("60min")

		// Save the file
		if err := cfg.SaveTo(configPath); err != nil {
			return fmt.Errorf("failed to create default config.ini: %w", err)
		}

		fmt.Println("Default config.ini created at:", configPath)
	}

	// Load configuration
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config.ini: %w", err)
	}

	// Parse configuration values
	musicDir = cfg.Section("user").Key("music_dir").String()
	if musicDir == "" {
		musicDir = defaultMusicDir
	}

	volume, err = cfg.Section("user").Key("volume").Float64()
	if err != nil || volume <= 0 {
		volume = defaultVolume
	}

	return nil
}

func initializePlaylist(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			switch filepath.Ext(path) {
			case ".mp3", ".wav", ".ogg", ".flac":
				files = append(files, path)
			}
		}
		return nil
	})

	return files, err
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
	// Open and decode the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
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
		fmt.Println("Unsupported file format:", ext)
		return
	}
	if err != nil {
		fmt.Println("Error decoding file:", err)
		return
	}
	defer musicStreamer.Close()

	if musicFormat.SampleRate == beep.SampleRate(defaultSampleRate) {
		controlStreamer = &beep.Ctrl{Streamer: musicStreamer, Paused: false}
	} else {
		controlStreamer = &beep.Ctrl{Streamer: beep.Resample(4, musicFormat.SampleRate, beep.SampleRate(defaultSampleRate), musicStreamer), Paused: false}
	}

	speaker.Clear()
	volumeControl := &effects.Volume{
		Streamer: controlStreamer,
		Base:     2,
		Volume:   float64(volume-100) / 16,
		Silent:   volume == 0,
	}
	done = make(chan bool)
	speaker.Play(beep.Seq(volumeControl, beep.Callback(func() {
		done <- true
	})))

	playingSong = true
	currentSongLabel.SetText("Now Playing: " + filepath.Base(filePath))
	playlistList.Refresh()                  // Refresh playlist display
	playlistList.ScrollTo(currentSongIndex) // Scroll to current song
	go updateProgressBar()

	<-done

	playingSong = false
	playPauseButton.SetText("Play")
	playNextSong() // Automatically play the next song

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
		}
	}
	speaker.Unlock()
	go updateProgressBar()
}

func adjustVolume(value float64) {
	speaker.Lock()
	volume = value
	if volumeControl != nil {
		volumeControl.Volume = float64(volume-100) / 16
	}
	speaker.Unlock()
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
