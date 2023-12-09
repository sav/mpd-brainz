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
   Alternatively, create a file `~/.mpd-brainz.conf` with a single line:

```yaml
listenbrainz_token: "<token>"
```

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

   
