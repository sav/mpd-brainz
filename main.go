/*
 * Copyright (c) 2023 Savio Sena <savio.sena@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
	"github.com/spf13/viper"
)

const ConfigFile = ".mpd-brainz.conf"

const listenBrainzURL = "https://api.listenbrainz.org/1/submit-listens"

type Info struct {
	MediaPlayer             string   `json:"media_player,omitempty"`
	MusicService            string   `json:"music_service,omitempty"`
	MusicServiceName        string   `json:"music_service_name,omitempty"`
	OriginUrl               string   `json:"origin_url,omitempty"`
	SubmissionClient        string   `json:"submission_client,omitempty"`
	SubmissionClientVersion string   `json:"submission_client_version,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
	Duration                int      `json:"duration,omitempty"`
}

type Track struct {
	Info        Info   `json:"additional_info,omitempty"`
	ArtistName  string `json:"artist_name,omitempty"`
	TrackName   string `json:"track_name,omitempty"`
	ReleaseName string `json:"release_name,omitempty"`
}

type Listen struct {
	ListenedAt int64 `json:"listened_at,omitempty"`
	Track      Track `json:"track_metadata,omitempty"`
}

type Listens struct {
	ListenType string   `json:"listen_type,omitempty"`
	Payload    []Listen `json:"payload,omitempty"`
}

var (
	lastListen Listens
	verbose    bool
	token      string
)

func main() {
	flag.BoolVar(&verbose, "v", false, "Enable debug logs.")
	flag.Parse()

	if verbose {
		log.Printf("(debug) verbose is %v\n", verbose)
	}

	viper.SetConfigName(ConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME")

	viper.SetDefault("mpd_address", "localhost:6600")
	viper.SetDefault("polling_interval_seconds", 10)
	viper.SetDefault("listenbrainz_token", "")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalf("Error opening configuration file: %s", err)
		}
	}

	mpdAddress := viper.GetString("mpd_address")
	interval := viper.GetDuration("polling_interval_seconds") * time.Second
	token = viper.GetString("listenbrainz_token")
	if token == "" {
		token = os.Getenv("LISTENBRAINZ_TOKEN")
	}
	if token == "" {
		log.Fatal(fmt.Sprintln("ListenBrainz token not found.",
			"Either define LISTENBRAINZ_TOKEN or set listenbrainz_token in",
			"~/"+ConfigFile+"."))
	}

	conn, err := mpd.Dial("tcp", mpdAddress)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	if verbose {
		log.Printf("(debug) connected to MPD: %s\n", mpdAddress)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if verbose {
		log.Printf("(debug) scrobbling with an interval of %s\n", interval)
	}

	for {
		select {
		case <-ticker.C:
			currentListen, err := getCurrentListen(conn)
			if err != nil {
				log.Println("error obtaining current song from MPD:", err)
				continue
			}
			if !currentListen.Equal(lastListen) && !currentListen.IsNil() {
				err = submitListen(currentListen, token)
				if err != nil {
					log.Println("error submitting scrobbles to ListenBrainz:", err)
					continue
				}
				currentListen.ListenType = "playing_now"
				currentListen.Payload[0].ListenedAt = 0
				err = submitListen(currentListen, token)
				if err != nil {
					log.Println("error submitting scrobbles to ListenBrainz:", err)
					continue
				}
				lastListen = currentListen
			} else {
			}
		case <-stop:
			return
		}
	}
}

func (l *Listens) IsNil() bool {
	return l == nil ||
		len(l.Payload) == 0 ||
		l.Payload[0].Track.ArtistName == "" ||
		l.Payload[0].Track.TrackName == ""
}

func (l *Listens) Equal(o Listens) bool {
	return l != nil && len(l.Payload) > 0 && len(o.Payload) > 0 &&
		l.Payload[0].Track.ArtistName == o.Payload[0].Track.ArtistName &&
		l.Payload[0].Track.TrackName == o.Payload[0].Track.TrackName
}

func getCurrentListen(conn *mpd.Client) (Listens, error) {
	currentSong, err := conn.CurrentSong()
	if err != nil {
		return Listens{}, err
	}

	musicService := currentSong["Name"]
	artistName := currentSong["Artist"]
	trackName := currentSong["Title"]
	releaseName := currentSong["Album"]
	originUrl := currentSong["file"]
	listenedAt := time.Now().Unix()

	// When receiving metadata in a unified field, particularly during online
	// radio playback, we attempt to parse and interpret it based on our
	// discoveries. As there isn't a set standard to ascertain the sequence,
	// the order we establish is essentially an inference from the data
	// received from these online sources. If inconsistencies arise with the
	// established orders, configuring customization in the file might be
	// necessary.

	if artistName == "" && strings.Contains(trackName, " - ") {
		elems := strings.Split(trackName, " - ")
		n := len(elems)
		switch n {
		case 2:
			artistName = elems[0]
			trackName = elems[1]
		case 4:
			fallthrough
		case 3:
			trackName = elems[0]
			artistName = elems[1]
			releaseName = elems[2]
		}
	}

	return Listens{
		ListenType: "single",
		Payload: []Listen{{
			ListenedAt: listenedAt,
			Track: Track{
				ArtistName:  artistName,
				TrackName:   trackName,
				ReleaseName: releaseName,
				Info: Info{
					SubmissionClient:        "scrobbler",
					SubmissionClientVersion: "0.1.0",
					MusicService:            musicService,
					OriginUrl:               originUrl,
				},
			},
		}},
	}, nil
}

func submitListen(listens Listens, token string) error {
	jsonData, err := json.MarshalIndent(listens, "", "   ")
	if err != nil {
		return err
	}

	if verbose && listens.ListenType == "single" {
		fmt.Printf("(debug) Submitting listen: %s - %s\n",
			listens.Payload[0].Track.ArtistName, listens.Payload[0].Track.TrackName)
	}

	req, err := http.NewRequest("POST", listenBrainzURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error submitting request. status: %s", resp.Status)
	}

	return nil
}
