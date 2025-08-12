# mpd-brainz

mpd-brainz is an MPD (Music Player Daemon) scrobbler designed to seamlessly integrate with ListenBrainz for accurate music listening tracking.

## Overview

This project aims to simplify the process of scrobbling music played through MPD to ListenBrainz. By utilizing this scrobbler, users can effortlessly contribute their listening habits to the ListenBrainz platform.

## Installation

To install mpd-brainz, run the following command:

```bash
go install github.com/sav/mpd-brainz@latest
```

## Configuration

Configuring mpd-brainz is straightforward:

1. **Environment Variable:**
   Set the environment variable `LISTENBRAINZ_TOKEN` with your ListenBrainz Authentication Token.

2. **Configuration File:**
   Copy the example configuration file to the proper location and customize it:

   ```bash
   mkdir -p ~/.config/mpd-brainz
   cp mpd-brainz.conf.example ~/.config/mpd-brainz/mpd-brainz.conf
   ```

   Then edit `~/.config/mpd-brainz/mpd-brainz.conf` with your settings:

   ```yaml
   mpd_address: "localhost:6600"
   mpd_password: ""
   polling_interval_seconds: 30
   listenbrainz_token: "<your_listenbrainz_token_here>"
   log_file: "~/.config/mpd-brainz/mpd-brainz.log"
   ```

   **Configuration Options:**
   - `mpd_address`: MPD server address and port (default: localhost:6600)
   - `mpd_password`: MPD server password if authentication is enabled
   - `polling_interval_seconds`: How often to check for new tracks (default: 30)
   - `listenbrainz_token`: Your ListenBrainz Authentication Token (required)
   - `log_file`: Path to the log file

Ensure you have a valid ListenBrainz Authentication Token to successfully scrobble your music.

## Scrobbling

Once running, mpd-brainz will automatically scrobble your MPD music playback to ListenBrainz. Simply start your MPD server, and mpd-brainz will handle the rest.

```bash
./mpd-brainz -v 
```

## Imports

To import your Shazam library exported as a CSV file into `mpd-brainz`, follow these steps:

   1. **Prepare your Shazam Library CSV:** 
      Export your Shazam library as a **CSV** file. Ensure the exported file contains the necessary fields or data that `mpd-brainz` expects. 

   2. **Execute the Import Command:**
      Use the following command syntax to import your Shazam library:

      ```bash
      ./mpd-brainz -i shazamlibrary.csv
      ```

## Contributing

Contributions to this project are welcome! Feel free to open issues for bug reports or suggest enhancements via pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

   
