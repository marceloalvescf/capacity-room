# Changelog

All notable changes to this project are recorded in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The list of reports shows the namespaces and the period of each report.

### Changed

- The web app is called CapacityRoom and has a look of its own, drawn from
  the nautical chart, in both themes: a new mark, typefaces, and colours,
  critical marked in magenta, the nodes of the cluster drawn against their
  maximum on the report overview, and the peak values that need attention
  tinted on the tables.
- The report overview opens on the number of findings of each severity and
  any warning about the quality of the data, then shows whether the cluster
  can grow and the pressure on each kind of resource, then the action plan,
  whose actions open their finding.
- The overview no longer counts the namespaces, workloads, HPAs, nodes, and
  node pools, and the Data quality tab shows its number of warnings.
- On a narrower screen the report overview keeps whether the cluster can
  grow beside the pressure on each kind of resource, lists the worst
  pressure first, and says when the evidence of the nodes is incomplete.
- The HPAs on the autoscaling tab show the status of their findings, as the
  namespaces, workloads, and nodes do.
- The nodes and the HPAs on the report say under each status why the row
  has it.
- The period of a new report is a field of its own, and its choices open
  below it in two columns without scrolling.
- Deleting a report names it in the confirmation, and a report that failed
  on its metric source points an administrator to the profile.
- The node pools on the report show the status of their findings, and the
  PDF choices line up.
- The autoscaling tab opens on the cluster's verdict, the action plan names
  the kind of each scope, and a failed report can be deleted.

### Fixed

- In the findings, the detail of a finding stays in view while the list
  scrolls, and a filter that matches nothing offers to show every finding.
- The workloads search also narrows the list of workloads with no observed
  usage.
- Status words and units no longer break in the middle of a word in narrow
  table cells.
- A date and its time stay on one line, and the thresholds of a profile read
  "70%" in both languages.
- On the report screen, a namespace, workload, or node with no finding reads
  "No finding" instead of Healthy, and the state of a node that is not ready
  or unknown carries its mark.
- A report whose metric source could not be reached no longer says that the
  source did not answer in time.
- Screen readers tell the actions of the action plan apart by their scope.
- A node whose requests take a high share of it marks that share on the
  report's nodes tab.

## [1.0.0] - 2026-10-10

### Added

- An operations guide explains how to install the platform behind a TLS
  proxy, configure it, back it up and restore it, upgrade it, size it, and
  read its failures.
- A security document describes whom the platform defends against, its
  controls, and the security review of this release.
- An operator who lost the last administrator's password resets it on the
  host with `admin reset-password`.

### Changed

- From this version on, an upgrade keeps the stored reports, and a change
  that would break the API, the command line, the settings files, or the
  snapshot file waits for a major version.
- The Compose stack runs on the fixed network `172.31.213.0/24`, so a TLS
  proxy on the host can be named in `trusted_proxies`.
- A credential or CA folder that holds the settings file or the server
  secret is refused at start.
- A flag written wrongly is refused with a fixed message that never repeats
  the text that was typed.

### Removed

- A database written by a version before 1.0.0 is not carried to this
  one: 1.0.0 starts from an empty database.

### Fixed

- A PDF attempt that resumes after it was replaced no longer overwrites the
  outcome of the attempt that replaced it.
- A profile test against an address that answers something other than HTTP
  reports the source as unreachable instead of failing with an error.

### Security

- The program is built with Go 1.27.2, which fixes ten vulnerabilities of
  the standard library in code the program calls.
- A password typed on the host and the server secret are hidden from the
  logs, as the other secrets are.
- The sign-in throttle tells clients apart by a canonical address, and an
  IPv6 client by its network, and a sign-in whose client has left no longer
  waits for its turn.
- The server secret must be a regular file of at most 64 KiB, and a
  credential or CA file is opened once inside its folder, so a file swapped
  while it is read cannot hold the worker.
- PostgreSQL runs without the right to gain privileges.

## [0.6.0] - 2026-10-09

### Added

- An administrator manages the metric-source profiles on the web: lists
  them, opens one in place, creates, edits, and deletes a profile, and tests
  its connection, saved or before saving it.
- An administrator manages the users on the web: creates a user, changes a
  user's role, language, or password, and deactivates or reactivates a user.

### Changed

- An opened row of a table scrolls into view and fits the window, with no
  sideways scrolling.
- A third click on a sorted column heading returns a table to its default
  order, and a search of a table announces the number of rows shown.

## [0.5.0] - 2026-10-08

### Added

- A collected report opens from its progress page and reads on the web in
  seven tabs (overview, findings, namespaces, workloads, autoscaling, nodes,
  and data quality), worded as its PDF, with its PDFs one click away.

### Changed

- The Compose stack runs as the project `k8s-capacity-report`, so it no
  longer shares its name with another stack on the same host.

### Fixed

- The development tools run on a NixOS host.

## [0.4.0] - 2026-10-07

### Added

- A report can cover a calendar range that has ended, such as September
  2026, typed or picked on a calendar in the form, or given with `--start`
  and `--end` on the command line.
- The form offers six calendar ranges: today so far, yesterday, this week so
  far, the previous week, this month so far, and the previous month, with
  weeks starting on Monday.

### Changed

- A report stored before this version can no longer be opened or given a new
  PDF; request it again.
- A snapshot file written by an earlier version can no longer be read by
  `--from-snapshot`; collect it again.

## [0.3.0] - 2026-10-07

### Added

- `serve` now serves a web interface, with a sign-in screen and an account
  screen where a reader changes their password and language.
- The interface is in Portuguese (Brazil) and English: it follows the
  browser's language before sign-in and the account's language after.
- The interface has a light and a dark theme, which follow the system unless
  the reader chooses one.
- A Compose file runs `serve`, `worker`, and PostgreSQL together.
- The interface lists the reports, newest first, and lets an operator delete
  their own reports, and an administrator any report, one at a time or
  several at once.
- The interface has a form that requests a report, with quick periods such
  as the last 24 hours and a typed period such as `36h` or `10d`.
- Each report has a progress page that follows it while it runs, downloads
  its PDF, and generates the PDF in the other language on request.

### Changed

- A report request names the language of its report, and only the PDF in
  that language is made at first; the PDF in the other language is made
  when someone asks for it.
- An expired report can be deleted, one at a time or several at once, where
  before the delete was refused.
- The Compose file gives `serve` and `worker` the time their stop needs
  before Docker ends them.

### Removed

- The API route that compiled a failed PDF again is gone; the route that
  requests the PDF of a language does that and more.

### Fixed

- Downloading a large report no longer leaves `serve` holding the memory of
  that download afterwards.
- Stopping `serve` no longer fails while a download is stalled: it waits 30
  seconds, closes what is left, and exits cleanly.

## [0.2.0] - 2026-10-06

### Added

- The `serve` command, the backend of the web platform, listens on port 8080
  and applies the database schema under a lock before it answers.
- Sign-in with local accounts in three roles, administrator, operator, and
  reader, with a session cookie that expires after twelve hours or one idle
  hour, and a CSRF token on every state-changing request.
- Five failed sign-ins in five minutes block the account and address pair
  for five minutes.
- A signed-in reader changes their own password and language through the
  API; a password change signs them out everywhere.
- An administrator lists, creates, changes, and deactivates users through
  the API; the last active administrator cannot be removed.
- Every sign-in, sign-out, bootstrap, and change to a user writes an audit
  row that an operator reads with SQL.
- The `admin migrate` command applies the database schema.
- The `admin bootstrap-admin` command creates the first administrator from a
  password on standard input, only with `KCR_BOOTSTRAP_MODE=1` and only
  while no user exists.
- The `admin probe` command checks a health address, for a container
  without `curl`.
- The health routes `/health/live` and `/health/ready`.
- The deployment file `platform.toml` and the `KCR_DATABASE_DSN` variable.
- The API contract, `api/openapi.yaml`.
- An administrator creates, reads, revises, deactivates, and deletes
  metric-source profiles through the API; every change is a new immutable
  revision, and a change made from an outdated reading is refused.
- Every signed-in reader lists the active profiles with their permitted
  periods, time zones, report profiles, and minimum severities.
- An administrator tests a profile against its source through the API and
  reads the outcome as a code, never an address or the text of a failure.
- Every request to a metric source passes an outbound policy: only the
  host names and address ranges the deployment file lists are reachable,
  the loopback, link-local, multicast, and metadata addresses are refused
  unless listed one by one, every resolved address is checked before each
  request, redirects are refused, and proxy variables are ignored.
- A profile names its credential and its CA bundle by file name, read from
  two folders the deployment file names, only when a profile is tested or
  a report runs.
- The `admin import-profiles` command creates or revises profiles from a
  TOML file and can be run again without effect.
- The `worker` command runs the report jobs of the platform, two at a time
  by default, and answers its health checks on port 8091.
- Sign-in records whose block is over, and sessions that ended longer than
  a session lifetime ago, are removed once a minute.
- An operator requests a report of a profile through the API with an
  idempotency key; the same key with the same request answers the same
  report, and a request over the platform's or the profile's admission
  limit is refused with the number of seconds to wait.
- Each requested report is collected through the outbound policy, stored as
  a snapshot, and compiled into one PDF in Portuguese (Brazil) and one in
  English by the worker.
- Every signed-in reader lists the reports page by page, newest first, and
  reads one with its request, its progress, and the state of each PDF.
- Every signed-in reader opens a report worded in Portuguese (Brazil) or
  English for the web, with the same texts and the same narrowing by
  severity as its PDF.
- Every signed-in reader downloads the snapshot of a report and its PDF in
  either language, and a download repeated with the tag of the copy already
  held answers that nothing changed.
- An operator asks again for the PDF of one language whose compilation
  failed.
- An operator deletes the reports it requested and an administrator any
  report, one at a time or up to 200 at once; a deleted report disappears
  at once and its data is removed within a minute.
- A report answers that it expired from the end of its retention until it
  is removed, and its snapshot and PDFs are deleted when it expires.

### Changed

- `admin import-profiles` reports `recreated`, and records it apart from a
  creation, for a profile that was deleted through the API.
- The outbound policy refuses three more IPv6 forms that embed an IPv4
  address.
- The profile test gives up after two minutes at most.
- A snapshot file read with `--from-snapshot` is refused when it repeats a
  key, holds text that is not UTF-8, or holds a combination of values that
  the program never writes.
- A `token=`, `api_key=`, or `access_token=` value is removed from a message
  up to the quote, brace, bracket, or parenthesis that closes it, which is
  kept, and a value written with escaped quotes is removed whole.

### Fixed

- A slow database write at the end of a collection or a compilation no
  longer fails the report as if its metric source were unreachable.

## [0.1.0] - 2026-10-02

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
- The report holds the capacity of each namespace and the resources of each
  workload: average and peak consumption of CPU and memory, the requests and
  the limits, and the peak of consumption judged against both.
- The report lists the interpretation notes of the collection after the data
  quality appendix.
- The snapshot file holds the namespaces, the workloads, the findings, the
  warnings, and the notices.
- A run that collects prints one line for the collection of the namespaces
  and one for the collection of the workloads.
- An answer of the endpoint larger than 256 MiB stops the run.
- A period that is empty, because the offset of the time zone changes inside
  it, stops the run with a message that names the period and the zone.
- A namespace of the configuration, or a workload that the endpoint answers,
  whose name holds a slash stops the run, because no name of Kubernetes
  holds one.
- The report holds the history of the HPAs of each namespace and the
  workloads without one, with the situation of each against its configured
  maximum.
- The report holds the usage of the node pools, a profile of allocatable,
  used, and requested resources per node, and the detail of each node with
  the reason of its status.
- The snapshot file holds the HPAs, the nodes, the node pools, the coverage
  of each workload, the cluster autoscaling evidence, and the counts of the
  report.
- A run that collects prints one line for each of the five collections and
  one for the cluster autoscaling.
- A run whose collectors return nothing, with the state of the cluster
  autoscaling unknown, stops with exit code 5.
- The report holds an executive summary, section 5: the workloads with
  critical and high priority, the namespaces with a workload at risk, the
  workloads at their replica maximum and without HPA, the situation of the
  node pools, the prioritized action plan, and the material limitations.
- The `executive` report profile writes the opening page, the executive
  summary, and the quality appendix, with the minimum severity raised to at
  least attention; the default name of its PDF ends in `-executive` in place
  of the severity.
- The snapshot file holds the recommendations and their count.

### Changed

- The Portuguese report writes the technical terms of Kubernetes in English:
  request, limit, workload, node, node pool, label, and autoscaling.
- Every table cell of the report, the header row included, is centred
  horizontally and vertically.
- The page margins of the report are one inch at the top and the sides and
  half an inch at the bottom.

### Fixed

- The average of a series equals the original's when the series has more
  than two samples.
- A threshold or a timeout that is not a finite number is refused by the
  configuration.
- The host of an IPv6 instance keeps the letter case of its zone when it is
  matched to a node.
- A name in the report metadata, such as a namespace, breaks only at its
  own hyphens, in Portuguese too, and never starts a line with a hyphen.
