package main

import (
    "github.com/faiface/beep"
    "github.com/faiface/beep/mp3"
    "github.com/faiface/beep/speaker"
    "os"
    "time"
)

func main() {
    // Open the MP3 file
    file, err := os.Open("example.mp3") // Replace with the path to your MP3 file
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Decode the MP3 file
    streamer, format, err := mp3.Decode(file)
    if err != nil {
        panic(err)
    }
    defer streamer.Close()

    // Initialize the speaker with the decoded format
    speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

    // Play the audio
    speaker.Play(beep.Seq(streamer, beep.Callback(func() {
        println("Playback finished")
    })))

    // Keep the program running while the audio plays
    select {}
}
