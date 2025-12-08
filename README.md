# Mobius Hotline Client

Mobius Hotline Client is a cross-platform command line [Hotline](https://en.wikipedia.org/wiki/Hotline_Communications) client implemented in Golang.

## Goals

* Provide a fully featured CLI Hotline for modern macOS, Linux, BSD, and Windows.
* Provide an example of how to use the [hotline](https://github.com/jhalter/mobius) package to build a client

## Project Status

| Feature                    | Done |
|----------------------------|------|
| Trackers listing           | ✓    |
| Connect to servers         | ✓    |
| Server accounts            | ✓    |
| Server bookmarks           | ✓    |
| Change name & icon         | ✓    |
| Display server agreement   | ✓    |
| Public chat                | ✓    |
| Private messages           | ~    |
| User list                  |      |
| User administration        |      |
| News reading               |      |
| News posting               |      |
| Message board reading      | ✓    |
| Message board posting      | ✓    |
| File browsing              | ✓    |
| File downloading           |      |
| File uploading             |      |
| File info                  |      |
| File management            |      |
| Folder downloading         |      |
| Folder uploading           |      |

## Usage

```
mobius-hotline-client [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | OS-dependent (see below) | Path to config file |
| `-log-level` | `info` | Log level (`debug` or `info`) |

### Config File Locations

The client looks for the config file in the following locations by default:

- **Linux**: `/usr/local/etc/mobius-client-config.yaml`
- **macOS**: `/usr/local/etc/mobius-client-config.yaml` or `/opt/homebrew/etc/mobius-client-config.yaml`
- **Windows**: `mobius-client-config.yaml` (current directory)

To use a config file in a different location:

```bash
mobius-hotline-client -config ./mobius-client-config.yaml
```

## Screenshots

<img width="837" alt="Screenshot 2024-07-21 at 4 14 51 PM" src="https://github.com/user-attachments/assets/b01d3deb-c8e0-46b4-9663-f94bc15fa0ec">
