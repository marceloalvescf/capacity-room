# Changelog

All notable changes to this project are recorded in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The `report` command checks a Prometheus-compatible endpoint, the cluster
  scope, and the metrics the endpoint offers, with `--check`.
- The `report` command writes a PDF with the cover, the report metadata, the
  status legend, and the data quality appendix, in Portuguese (Brazil) or
  English.
- The `report` command stores what it found in a snapshot file with
  `--snapshot`, and writes a PDF from that file with `--from-snapshot`,
  without any request to the endpoint.
- The configuration is read from a TOML file, then from the environment, then
  from the command line.
- The bearer token is taken from an environment variable or from `--token`,
  and is removed from every message the program prints.
- A bearer token shorter than 8 characters is refused, because a text that
  short cannot be told from ordinary words in a message.
