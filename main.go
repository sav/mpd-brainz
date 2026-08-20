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
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/fhs/gompd/mpd"
	"github.com/spf13/viper"
)

const ConfigDir = "mpd-brainz"
const ConfigFile = "mpd-brainz.conf"
const DefaultLogFile = "mpd-brainz.log"
const ListenBrainzURL = "https://api.listenbrainz.org/1/submit-listens"

//go:embed VERSION
var Version string

func version() {
	fmt.Printf("mpd-brainz v%s", Version)
	os.Exit(0)
}

var Logger *log.Logger = log.New(os.Stdout, "", log.LstdFlags)

func Log(fmt string, args ...any) {
	Logger.Printf(fmt+"\n", args...)
}

func Debug(fmt string, args ...any) {
	if verbose {
		Logger.Printf(fmt+"\n", args...)
	}
}

func Error(fmt string, args ...any) {
	Logger.Printf("error: "+fmt+"\n", args...)
}

func Fatal(fmt string, args ...any) {
	Logger.Fatalf("error: "+fmt+"\n", args...)
}

type Info struct {
	MediaPlayer             string   `json:"media_player,omitempty"`
	MusicService            string   `json:"music_service,omitempty"`
	MusicServiceName        string   `json:"music_service_name,omitempty"`
	OriginUrl               string   `json:"origin_url,omitempty"`
	SubmissionClient        string   `json:"submission_client,omitempty"`
	SubmissionClientVersion string   `json:"submission_client_version,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
	Duration                int      `json:"duration,omitempty"`
	ArtistMBIDs             []string `json:"artist_mbids,omitempty"`
	RecordingMBID           string   `json:"recording_mbid,omitempty"`
	ReleaseMBID             string   `json:"release_mbid,omitempty"`
	ReleaseGroupMBID        string   `json:"release_group_mbid,omitempty"`
	TrackMBID               string   `json:"track_mbid,omitempty"`
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

func (l *Listen) String() string {
	return fmt.Sprintf("\"%s - %s\"", l.Track.ArtistName, l.Track.TrackName)
}

type Listens struct {
	ListenType string   `json:"listen_type,omitempty"`
	Payload    []Listen `json:"payload,omitempty"`
}

const ListensMaxSize = 500

func NewListens(listenType string) Listens {
	return Listens{
		ListenType: listenType,
		Payload:    []Listen{},
	}
}

func NewListen(listenType string, artistName string, trackName string,
	releaseName string, originUrl string, musicService string, timestamp int64) Listens {
	listens := NewListens("single")
	listens.Add(artistName, trackName, releaseName, originUrl, musicService, timestamp)
	return listens
}

func (l *Listens) Length() int {
	return len(l.Payload)
}

func (l *Listens) String() string {
	s := ""
	n := l.Length()
	if n == 1 {
		return l.Payload[0].String()
	}
	for i := 0; i < n; i++ {
		t := l.Payload[i].String()
		if i != n-1 {
			t += ", "
		}
		s += t
	}
	return fmt.Sprintf("{%s, [%s]}", l.ListenType, s)
}

func (l *Listens) IsNil() bool {
	return l == nil ||
		l.Length() == 0 ||
		l.Payload[0].Track.ArtistName == "" ||
		l.Payload[0].Track.TrackName == ""
}

func (l *Listens) Equal(o Listens) bool {
	return l != nil && l.Length() > 0 && o.Length() > 0 &&
		l.Payload[0].Track.ArtistName == o.Payload[0].Track.ArtistName &&
		l.Payload[0].Track.TrackName == o.Payload[0].Track.TrackName
}

func (l *Listens) Add(artistName string, trackName string, releaseName string,
	originUrl string, musicService string, listenedAt int64) {
	if listenedAt == 0 {
		listenedAt = time.Now().Unix()
	}

	// When receiving metadata in a unified field, particularly during online
	// radio playback, we attempt to parse and interpret it based on our
	// discoveries. As there isn't a set standard to ascertain the sequence,
	// the order we establish is essentially an inference from the data
	// received from these online sources. If inconsistencies arise with the
	// established orders, it might be necessary to allow proper customization
	// in the configuration file.

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

	l.Payload = append(l.Payload, Listen{
		ListenedAt: listenedAt,
		Track: Track{
			ArtistName:  artistName,
			TrackName:   trackName,
			ReleaseName: releaseName,
			Info: Info{
				SubmissionClient:        "mpd-brainz",
				SubmissionClientVersion: Version,
				MusicService:            musicService,
				OriginUrl:               originUrl,
			},
		},
	})
}

func (l *Listens) OriginUrl() string {
	if l.Length() == 0 {
		return ""
	}
	return l.Payload[0].Track.Info.OriginUrl
}

func (l *Listens) SetListenedAt(listenedAt int64) {
	if l.Length() > 0 {
		l.Payload[0].ListenedAt = listenedAt
	}
}

func (l *Listens) SetDuration(duration time.Duration) {
	if l.Length() > 0 && duration > 0 {
		l.Payload[0].Track.Info.Duration = int(duration.Seconds())
	}
}

func (l *Listens) Submit(listenType string, token string) error {
	l.ListenType = listenType
	if l.ListenType == "playing_now" {
		// A "playing now" listen carries no timestamp.
		l.SetListenedAt(0)
	} else if l.ListenType == "import" {
		Log("importing %d listens", l.Length())
	} else {
		Log("submitting listen: %s", l)
	}

	jsonData, err := json.MarshalIndent(l, "", "   ")
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", ListenBrainzURL, bytes.NewBuffer(jsonData))
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

	if resp.StatusCode == http.StatusBadRequest {
		Debug("bad request with data: %s", jsonData)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error submitting request. status: %s", resp.Status)
	}

	return nil
}

// SetMusicBrainzIDs copies the MusicBrainz identifiers MPD reads from the
// file tags into the last listen added. Beware of the tag naming inherited
// from Picard: MUSICBRAINZ_TRACKID holds the recording MBID, whereas the
// track MBID lives in MUSICBRAINZ_RELEASETRACKID.
func (l *Listens) SetMusicBrainzIDs(song mpd.Attrs) {
	n := l.Length()
	if n == 0 {
		return
	}

	info := &l.Payload[n-1].Track.Info
	info.RecordingMBID = song["MUSICBRAINZ_TRACKID"]
	info.TrackMBID = song["MUSICBRAINZ_RELEASETRACKID"]
	info.ReleaseMBID = song["MUSICBRAINZ_ALBUMID"]
	info.ReleaseGroupMBID = song["MUSICBRAINZ_RELEASEGROUPID"]
	if artistMBID := song["MUSICBRAINZ_ARTISTID"]; artistMBID != "" {
		info.ArtistMBIDs = []string{artistMBID}
	}
}

func getCurrentListen(conn *mpd.Client) (Listens, error) {
	currentSong, err := conn.CurrentSong()
	if err != nil {
		return Listens{}, err
	}

	artistName := currentSong["Artist"]
	trackName := currentSong["Title"]
	releaseName := currentSong["Album"]
	originUrl := currentSong["file"]
	musicService := currentSong["Name"]

	listens := NewListen("single", artistName, trackName, releaseName,
		originUrl, musicService, 0)
	listens.SetMusicBrainzIDs(currentSong)

	return listens, nil
}

// A listen is only submitted once we have watched enough of the song
// actually play, which is ListenBrainz's own recommendation: half of the
// track, or four minutes, whichever comes first.
const ListenThreshold = 4 * time.Minute

// Online radio reports no duration, leaving no track length to halve.
// Require a short minimum instead.
const OnlineRadioThreshold = time.Minute

func listenThreshold(duration time.Duration) time.Duration {
	if duration <= 0 {
		return OnlineRadioThreshold
	}
	return min(duration/2, ListenThreshold)
}

func seconds(value string) time.Duration {
	if value == "" {
		return 0
	}
	s, err := strconv.ParseFloat(value, 64)
	if err != nil {
		Debug("parsing seconds: %q: %s", value, err)
		return 0
	}
	return time.Duration(s * float64(time.Second))
}

// Playback follows the song MPD currently sits on, along with how much of
// it we have watched play. MPD reports the current song whether it is
// playing, paused or stopped, so the playback we observe is what tells a
// real listen apart from a queue that was restored on start-up.
type Playback struct {
	listen     Listens
	songID     string
	originUrl  string
	listenedAt int64
	duration   time.Duration
	elapsed    time.Duration
	played     time.Duration
	announced  bool
	submitted  bool
	polledAt   time.Time
}

func (p *Playback) poll(conn *mpd.Client, conf Config) error {
	status, err := conn.Status()
	if err != nil {
		return fmt.Errorf("obtaining status from MPD: %s", err)
	}

	currentListen, err := getCurrentListen(conn)
	if err != nil {
		return fmt.Errorf("obtaining current song from MPD: %s", err)
	}

	now := time.Now()
	polledAt := p.polledAt
	p.polledAt = now

	playing := status["state"] == "play"
	elapsed := seconds(status["elapsed"])
	songID := status["songid"]
	originUrl := currentListen.OriginUrl()

	if currentListen.IsNil() {
		*p = Playback{polledAt: now}
		return nil
	}

	// MPD hands out a fresh song id every time it starts playing a song,
	// repeats included, so it tells a new listen from a seek within the
	// one we are already following. Online radio is the exception: the
	// station is a single queue entry whose id and file never change, and
	// only the metadata says the song did.
	if songID != p.songID || originUrl != p.originUrl ||
		!currentListen.Equal(p.listen) {
		*p = Playback{
			listen:     currentListen,
			songID:     songID,
			originUrl:  originUrl,
			listenedAt: now.Unix(),
			duration:   seconds(status["duration"]),
			polledAt:   now,
		}
		p.listen.SetDuration(p.duration)
		// Credit the playback that happened before this poll, but never
		// more than one interval of it: anything longer is a position MPD
		// restored, not playback we watched. After a gap we watched
		// nothing at all.
		if playing && !polledAt.IsZero() {
			p.played = min(elapsed, conf.interval)
		}
		Debug("current song: %s (duration: %s, threshold: %s)",
			&p.listen, p.duration, listenThreshold(p.duration))
	} else {
		// MPD leaves elapsed and duration out of the status of a stopped
		// song, so a queue we first saw stopped has no length yet.
		if p.duration == 0 {
			p.duration = seconds(status["duration"])
			p.listen.SetDuration(p.duration)
		}
		// MPD's elapsed time only advances while it plays, so it caps
		// what wall time alone would credit across a gap: a paused song
		// between two polls, a suspended machine, a stalled stream.
		if playing && !polledAt.IsZero() {
			if progress := min(now.Sub(polledAt), elapsed-p.elapsed); progress > 0 {
				p.played += progress
			}
		}
	}
	p.elapsed = elapsed

	if !playing {
		Debug("MPD is not playing (state: %s), holding %s of %s",
			status["state"], p.played, listenThreshold(p.duration))
		return nil
	}

	if !p.announced {
		if err := p.listen.Submit("playing_now", conf.token); err != nil {
			Error("submitting \"playing now\" to ListenBrainz: %s", err)
		} else {
			p.announced = true
		}
	}

	if !p.submitted && p.played >= listenThreshold(p.duration) {
		// Submitting "playing now" drops the timestamp, so restore the
		// moment playback started before submitting the listen itself.
		p.listen.SetListenedAt(p.listenedAt)
		if err := p.listen.Submit("single", conf.token); err != nil {
			return fmt.Errorf("submitting scrobble to ListenBrainz: %s", err)
		}
		p.submitted = true
	}

	return nil
}

// MPD can go away at any time — the user quits it, the service restarts —
// and gompd never dials again on its own, leaving every command failing
// forever. The ping doubles as the keepalive MPD expects within its
// connection_timeout.
func connect(conn *mpd.Client, conf Config) (*mpd.Client, bool) {
	if conn != nil {
		if err := conn.Ping(); err == nil {
			return conn, false
		}
		Error("lost connection to MPD: %s", conf.mpdAddress)
		conn.Close()
	}

	next, err := mpd.DialAuthenticated("tcp", conf.mpdAddress, conf.mpdPassword)
	if err != nil {
		Debug("reconnecting to MPD: %s", err)
		return nil, false
	}
	Log("reconnected to MPD: %s", conf.mpdAddress)

	return next, true
}

func scrobble(conf Config) {
	conn, err := mpd.DialAuthenticated("tcp", conf.mpdAddress, conf.mpdPassword)
	if err != nil {
		Fatal("%s", err)
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	Log("connected to MPD: %s", conf.mpdAddress)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	ticker := time.NewTicker(conf.interval)
	defer ticker.Stop()
	Debug("scrobbling interval: %s", conf.interval)

	var playback Playback

	for {
		select {
		case <-ticker.C:
			var reconnected bool
			if conn, reconnected = connect(conn, conf); conn == nil {
				continue
			}
			if reconnected {
				// Whatever happened while we were away is not playback
				// we watched.
				playback.polledAt = time.Time{}
			}
			if err := playback.poll(conn, conf); err != nil {
				Error("%s", err)
			}
		case <-stop:
			return
		}
	}
}

func skipLine(file *os.File) {
	info, err := file.Stat()
	if err != nil {
		Fatal("reading file stats: %s: %s", file.Name, err)
	}

	var n int = int(info.Size())
	var b []byte = []byte{' '}

	for i := 0; i < n; i++ {
		_, err = file.Read(b)
		if b[0] == '\n' {
			break
		}
	}
}

func dateToUnix(date string) int64 {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		Error("parsing date: %s: %s", date, err)
		return 0
	}
	return t.Unix()
}

func shazamBuffListens(reader *csv.Reader, listen *Listens) bool {
	for i := 0; i < ListensMaxSize; i++ {
		e, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				return true
			}
			Error("%s", err)
			i -= 1
			continue
		}
		listen.Add(e[3], e[2], "", e[4], "shazam.com", dateToUnix(e[1]))
	}
	return false
}

func shazam(conf Config) {
	file, err := os.Open(importShazam)
	if err != nil {
		Fatal("opening file: %s", err)
	}
	defer file.Close()

	skipLine(file)
	skipLine(file)

	reader := csv.NewReader(file)
	for {
		listens := NewListens("import")
		finished := shazamBuffListens(reader, &listens)
		err = listens.Submit("import", conf.token)
		if err != nil {
			Fatal("submitting \"import\" to ListenBrainz: %s", err)
		}
		if finished {
			break
		}
	}
}

func setLog(rootDir string, logConf string) {
	if logPath == "" {
		if logConf == "" {
			logPath = filepath.Join(rootDir, DefaultLogFile)
		} else {
			logPath = logConf
		}
	}

	var logFile *os.File
	var err error

	if logPath == "-" {
		logFile = os.Stdout
	} else {
		logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			Error("opening log file: %s", err)
			logFile = os.Stdout
		}
	}

	Debug("writing logs to file: %s", logFile.Name())
	Logger = log.New(logFile, "", log.LstdFlags)
	if Logger == nil {
		Fatal("failed creating logger: %s", logPath)
	}
}

type Config struct {
	mpdAddress  string
	mpdPassword string
	interval    time.Duration
	token       string
}

func findConfig() (string, string) {
	configRoot := ""
	configFile := ConfigFile

	if configPath == "" {
		configRoot = filepath.Join(os.Getenv("XDG_CONFIG_HOME"), ConfigDir)
		if configRoot == ConfigDir {
			configRoot = filepath.Join(os.Getenv("HOME"), ".config", ConfigDir)
		}

		err := os.Chdir(configRoot)
		if err == os.ErrNotExist {
			err = os.Mkdir(configRoot, 0700)
		}
		if err != nil {
			Error("can't access config directory: %s", configRoot)
			configRoot = ""
		}
	} else {
		configAbs, err := filepath.Abs(configPath)
		if err != nil {
			Error("invalid file path: %s", configPath)
		} else {
			configPath = configAbs
		}
		configRoot = filepath.Dir(configPath)
		configFile = filepath.Base(configPath)
	}

	return configRoot, configFile
}

func config() Config {
	configRoot, configFile := findConfig()
	viper.AddConfigPath(configRoot)
	viper.SetConfigName(configFile)
	viper.SetConfigType("yaml")

	viper.SetDefault("mpd_address", "localhost:6600")
	viper.SetDefault("mpd_password", "")
	viper.SetDefault("polling_interval_seconds", 10)
	viper.SetDefault("listenbrainz_token", "")
	viper.SetDefault("log_file", "")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			Fatal("invalid configuration file: %s: %s", viper.ConfigFileUsed(), err)
		} else {
			Error("opening configuration file: %s: %s", viper.ConfigFileUsed(), err)
		}
	}
	Debug("loading configuration: %s", viper.ConfigFileUsed())

	var conf Config
	conf.mpdAddress = viper.GetString("mpd_address")
	conf.mpdPassword = viper.GetString("mpd_password")
	conf.interval = viper.GetDuration("polling_interval_seconds") * time.Second
	conf.token = viper.GetString("listenbrainz_token")

	if conf.token == "" {
		conf.token = os.Getenv("LISTENBRAINZ_TOKEN")
	}
	if conf.token == "" {
		Fatal(fmt.Sprintln("ListenBrainz token not found.",
			"Either define LISTENBRAINZ_TOKEN or set listenbrainz_token in",
			viper.ConfigFileUsed()+"."))
	}

	setLog(configRoot, viper.GetString("log_file"))
	return conf
}

var (
	verbose      bool
	printVersion bool
	importShazam string
	configPath   string
	logPath      string
)

func optarg() {
	flag.BoolVar(&verbose, "v", false, "Enable debug logs.")
	flag.BoolVar(&printVersion, "V", false, "Print version number.")
	flag.StringVar(&importShazam, "i", "", "Import Shazam Library.")
	flag.StringVar(&logPath, "l", "", "Set log file.")
	flag.StringVar(&configPath, "c", "", "Config file.")
	flag.Parse()
}

func main() {
	optarg()
	if printVersion {
		version()
	}

	conf := config()
	if importShazam != "" {
		shazam(conf)
	} else {
		scrobble(conf)
	}
}
