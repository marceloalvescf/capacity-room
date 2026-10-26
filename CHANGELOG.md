# Changelog

All notable changes to this project are recorded in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [4.0.0] - 2026-10-26

### Changed

- The upgrade to this version carries every stored report and profile forward and cannot be undone, so back up the database and stop the worker before it, as the operations guide says.

### Removed

- Grafana is no longer a metric source: a source is the URL of a Prometheus-compatible API, such as Prometheus, Thanos Query Frontend, VictoriaMetrics, or Cortex.
- The endpoint type is gone: the `--endpoint-type` flag, `endpoint_type` in `report.toml`, in a profile, in the import file, and in the chart's profiles, and the field of a profile in the API are now refused, and the `CAPACITYROOM_ENDPOINT_TYPE` variable is no longer read.
- A snapshot file written by 3.x is refused, and the operations guide shows the one edit that converts it.

## [3.1.0] - 2026-10-26

### Added

- In every table whose rows open, J or Down moves the focus to the next row and K or Up to the previous one, while a row's open button has the focus.
- The Helm chart is published as the OCI chart `oci://ghcr.io/marceloalvescf/charts/capacity-room` with its examples, and the guide installs it from the chart's folder.

### Security

- Sign-ins from one client address check one password at a time, so an address that tries many user names can no longer hold back the sign-ins of others.

## [3.0.0] - 2026-10-25

### Changed

- A report shows the findings its profile shows, and every count of the PDF and of the web counts only those: an executive report counts its critical and attention findings, and a report that shows none says "Nothing needs attention."
- The upgrade to this version carries every stored report and profile forward and cannot be undone, so back up the database and stop the worker before it, as the operations guide says.
- The PDF says "Every capability of this report is available." when its appendix has nothing to list, and "No workload has observed usage in this period." when no workload has a number to show.

### Removed

- The minimum severity is gone: the `--min-severity` flag, `min_severity` in `report.toml`, a profile's default and permitted minimum severities, and the field of a report request are now refused, and the `CAPACITYROOM_MIN_SEVERITY` variable is no longer read.
- The opening page of the PDF no longer lists a minimum severity, and the name of a PDF no longer carries one.
- A snapshot file written by 2.x is refused, and the operations guide shows the one edit that converts it.

### Fixed

- A node pool or an HPA on the web takes the status of all its findings, as the PDF does, so a pool whose findings are all healthy no longer reads "No finding".
- The bars of each scope on the web no longer show a healthy count that an executive report does not show.

### Security

- The web app checks its forms with Zod 4.6.7, which no longer exhausts memory on an oversized list.

## [2.8.0] - 2026-10-21

### Added

- The home page shows every cluster by its latest report, and answers the chosen cluster with its totals, its pressure, its growth, and what to do first.
- A report can be requested for a cluster straight from the home page.

## [2.7.0] - 2026-10-21

### Added

- The report's overview, its Findings tab, and the first page of the PDF chart the findings: a ring of every finding by severity, and a bar of each scope's findings on one scale.
- A bar of the Findings tab opens the findings of that scope and severity.
- The Findings tab folds its findings under one row per severity, which opens in place.
- The Account page offers the browser's language as a choice beside the two languages; a reader who picks it sees the app in their browser's language after signing in. Existing accounts keep the language they had.
- The node names of a pool open from their count on the Nodes tab, each with a copy button, and all of them can be copied at once.

### Changed

- The image on Docker Hub is now public, so the chart's Gateway example no longer names a pull secret.
- The PDF uses the status colours of the web app on its marks, charts, gauge, and the cells that carry a status; words and numbers stay black.
- The spread of a node pool's peaks is drawn as a range, from its lowest node to its highest, with both ends named.
- `admin bootstrap-admin --language` and the chart's `admin.language` are optional; when left out, the first administrator follows the browser's language.
- A new user created without a language follows the browser's language.

### Security

- The program is built with Go 1.27.3, which fixes a weakness in how certificates are checked when it connects to a metric source over HTTPS.
- The image moves to the Debian 13 base, whose list of trusted certificate authorities is current.

## [2.6.0] - 2026-10-19

### Added

- The Helm chart gives its two setup Jobs resource requests and a memory limit, which the new `jobs.resources` value changes, so a cluster that requires limits on every container accepts them.
- The notes printed after a Helm install warn when no first administrator is created.

### Changed

- When a metric source resolves to an address that the installation does not allow, the message now names the two settings that allow it.
- The chart's quick start and the operations guide name the settings that every first install needs, and show how to find a gateway's address range on AKS.

### Fixed

- The chart's Ingress example now creates the first administrator and a profile, as the Gateway example does.
- The CloudNativePG example now applies in any namespace.

## [2.5.0] - 2026-10-16

### Added

- The web app shows the CapacityRoom mark as its icon in the browser tab, in light and dark.

### Changed

- An empty cell of the PDF now shows a hyphen instead of a long dash, so it no longer looks like the mark for data that is unavailable.

## [2.4.0] - 2026-10-16

### Changed

- Node autoscaling is now read from kube-state-metrics, as kube-prometheus-stack collects it, so a report on a cluster whose autoscaler is managed, as on AKS, or not scraped, as on OKE, says whether node autoscaling is enabled or disabled instead of saying it could not be determined.
- A report no longer shows the autoscaler's minimum and maximum nodes, the node pools it manages, or the gauge of nodes against the maximum, which kube-state-metrics does not hold; a report made before keeps them.
- "Scale events" now reads "Nodes added" and counts every node added during the period.
- A count of node autoscaling that is not known is left out of the report instead of reading Unavailable.

## [2.3.0] - 2026-10-15

### Changed

- The web app has a new look: neutral greys with blue kept for actions and links, softer corners, and Mona Sans for words, while names and numbers stay in JetBrains Mono.
- The report overview opens on the severity totals at a larger size, then shows the worst finding of each scope beside a bar that draws its evidence against its limit, request, capacity, or maximum.
- The nodes of the cluster against the autoscaler's maximum are drawn as the same bar, divided per node, in place of the depth gauge.
- The findings detail and the findings under an opened table row show the same evidence bar.
- A peak cell in attention or critical is tinted in the colour of its status, in place of the blue depth tint and the hatch.
- On the Findings tab a selected severity shows a dark outline and its coloured shape, in place of a blue fill.
- Page titles are larger, and menus, popovers, and dialogs float on a soft shadow.
- The current page and tab are marked in ink rather than blue, a status word keeps the colour of the text beside its coloured shape, and italic numbers are gone.

## [2.2.0] - 2026-10-15

### Changed

- Buttons, navigation links, report tabs, and menu items now show where the pointer is, and a button sinks slightly when pressed.
- Menus, popovers, tooltips, and dialogs now fade in and out, and open at once when the system asks for reduced motion.
- The interface blue is now clearly blue in both themes and fills primary buttons and chosen options, and the dark theme draws its frames and table rules more clearly.
- An empty cell in the web app now shows a hyphen instead of a long dash.
- A line of three or more items in the web app, such as the severity totals and the summary of a report, now parts them with space instead of dots.

## [2.1.1] - 2026-10-15

### Added

- The Helm chart has a README with a quick start, and the operations guide shows how to run its database on CloudNativePG.

## [2.1.0] - 2026-10-14

### Added

- A Helm chart runs the platform on Kubernetes, with a Gateway API route or an Ingress, and creates the first administrator and the profiles from its values.
- The first administrator can be created from a password file, with `admin bootstrap-admin --password-file`.

### Changed

- The Compose stack mounts its settings folder at `/opt/capacity-room`, with the server secret in a `secret` folder of its own; move the folder and edit its paths before upgrading, as the operations guide says.

## [2.0.0] - 2026-10-14

### Changed

- The program is now named CapacityRoom, and its command is now `capacity-room`.
- Every environment variable now starts with `CAPACITYROOM_`.
- The last line of the PDF names CapacityRoom.
- The Compose stack, its database, its user, and its configuration folder take the new name, and the operations guide says to write the new environment file afresh.
- A profile that is revised or imported again after the upgrade writes one new revision, even when its document is unchanged.

### Removed

- Reports stored by a Compose stack of an earlier version are not carried forward, since the new stack starts with a new volume.
- Every signed-in session ends at the upgrade, and a theme or a language chosen before sign-in is forgotten once.

## [1.6.0] - 2026-10-14

### Added

- The report list shows the cluster of each report under its profile, and the worst severity the report shows under its status; a report collected before this version shows neither.
- The report list names the language of each report, and the language of each of its PDFs.
- The API's report summary carries the worst severity and the cluster name of a collected report, or null when the report holds neither.
- The new report form says when a report it copies names a profile, namespaces, or a period that no longer apply.
- The action plan, a finding, and the cluster autoscaling explain what their confidence means.

### Changed

- The report list shows the date a report is kept until under its request time, so the list fits a window 1024 pixels wide.
- The title of a report reads "Report · " followed by the profile's name.
- A collection shorter than one second reads "under 1 s".
- The progress page counts the elapsed time from the start of the collection, and shows none while the report waits in the queue.
- The progress page offers Delete for every report the list lets you delete.
- The period picker no longer highlights a quick range while a range is being typed.

### Fixed

- A request that gets no answer from the server ends with a message, so a delete no longer leaves its dialog stuck; a profile test and a report's view wait up to two and a half minutes, every other request 30 seconds.
- A bulk delete cut short announces the reports that went and keeps the others selected.
- The report list no longer flashes profile ids while the profile names load, and says when they could not be loaded.
- The progress page checks often again after a PDF is requested, and announces nothing about a report you left.
- A link to a finding scrolls it into view.
- Screen readers tell apart the Open, Delete, and selection controls of each report in the list.
- A tooltip lies inside the region of the page that holds its button.
- The new report form keeps its submit button disabled while it opens the requested report.
- When the server refuses a selection the profile does not permit, the new report form loads the profile's current choices.
- An unknown report tab shows a single main heading.
- The API contract describes the view's minimum severity and the status of each namespace, workload, and node again.

## [1.5.0] - 2026-10-13

### Added

- A node that was not ready, or was cordoned, at the end of the period has a finding of attention, in the PDF and on the web.
- The State column of the nodes marks a node that is not ready or cordoned, and one whose readiness is unknown, in the PDF and on the web.

### Changed

- An HPA's situation carries the mark of its worst finding, so an HPA below its maximum with a finding of attention no longer reads healthy.
- Node utilization reads Partial while the evidence of node capacity is incomplete.
- The limit findings describe the usage against the limit and no longer predict a risk beside a healthy mark.
- The node autoscaling finding says the cluster reached or came close to its observed maximum.
- Every date of the report, in the PDF and on the web, is written as an ISO date and time.
- A healthy finding shows no action, and a capability with no remark shows an empty cell in place of "No remark".
- CPU variation is judged only from an average of 1 mCPU, so a near-zero average no longer reports an absurd variation.
- A report collected before this version shows a node that was not ready without a mark, since that report holds no readiness finding.

### Fixed

- A number and its unit no longer split across two lines in the PDF.
- A status mark no longer sits on a line above its label in a narrow column of the PDF.

## [1.4.0] - 2026-10-13

### Changed

- A report whose metric source's host name does not resolve is tried again, as a source that does not answer is.
- A second interrupt ends `serve`, `worker`, or a command at once while it is stopping.

### Fixed

- A metric source that writes sample values as numbers is read, as the original program reads it.
- A report whose source sends values too large to compute with fails with a reason of its own, no longer as an internal failure.
- A change request with both a wrong id and a wrong body names the body's fault, as every other route does.

### Security

- The worker reads a large answer of a metric source one series at a time, with far less memory.
- Opening the session address from another site no longer changes the token of an open page.

## [1.3.0] - 2026-10-13

### Added

- The report view of the API gives the verdict of the overview: the worst finding of the cluster, the node gauge, the pressure on each scope, and the finding each action comes from.

### Changed

- The web app names a node pool it could not identify "Undetermined", as the PDF does.
- The node gauge of the web app marks the minimum in whole nodes, as the PDF does.

### Fixed

- An action that covers more than three scopes no longer repeats their kind, in the PDF and in the web app.
- Screen readers read the status of every mark in the PDF as a word.

## [1.2.0] - 2026-10-13

### Added

- The PDF opens on a verdict page: the findings by severity, whether the cluster can grow, the pressure on each scope, and the first actions to take.
- The PDF ends with a line naming the program that generated it.

### Changed

- The executive summary of the PDF comes before the sections of evidence.
- The PDF uses the typefaces of the web app, with names and numbers set in a monospaced face and numbers aligned to the right.
- The PDF words its statuses, headings, and action plan as the web app does, and names each scope of an action with its kind and in full.
- The opening page of the PDF names the profile and the minimum severity of the report.
- The scope text of an action in the API names each of up to three scopes in full.
- An empty table of the PDF says whether there was no data or no row reached the minimum severity.

### Fixed

- The PDF no longer marks unavailable data, a partial HPA coverage, or a node that is not ready with the mark of a healthy or critical status.
- The data quality tab of the web app marks an unavailable capability as unavailable rather than critical.
- The PDF and the web app write "1 core" in the singular.

## [1.1.0] - 2026-10-12

### Added

- The list of reports shows the namespaces and the period of each report.

### Changed

- The web app is called CapacityRoom and has a look of its own, drawn from
  the nautical chart, in both themes.
- The web app has a new mark, typefaces, and colours, and critical is marked
  in magenta.
- The report overview draws the nodes of the cluster against their maximum.
- The tables tint the peak values that need attention.
- The report overview opens on the number of findings of each severity and
  any warning about the quality of the data.
- The report overview shows whether the cluster can grow and the pressure on
  each kind of resource.
- The report overview ends with the action plan, whose actions open their
  finding.
- The overview no longer counts the namespaces, workloads, HPAs, nodes, and
  node pools.
- The Data quality tab shows its number of warnings.
- On a narrower screen the report overview keeps whether the cluster can
  grow beside the pressure on each kind of resource.
- On a narrower screen the report overview lists the worst pressure first.
- The report overview says when the evidence of the nodes is incomplete.
- The HPAs on the autoscaling tab show the status of their findings, as the
  namespaces, workloads, and nodes do.
- The nodes and the HPAs on the report say under each status why the row
  has it.
- The period of a new report is a field of its own, and its choices open
  below it in two columns without scrolling.
- Deleting a report names it in the confirmation.
- A report that failed on its metric source points an administrator to the
  profile.
- The node pools on the report show the status of their findings.
- The PDF choices of a report line up.
- The autoscaling tab opens on the cluster's verdict.
- The action plan names the kind of each scope.
- A failed report can be deleted.

### Fixed

- In the findings, the detail of a finding stays in view while the list
  scrolls.
- A filter in the findings that matches nothing offers to show every
  finding.
- The workloads search also narrows the list of workloads with no observed
  usage.
- Status words and units no longer break in the middle of a word in narrow
  table cells.
- A date and its time stay on one line.
- The thresholds of a profile read "70%" in both languages.
- On the report screen, a namespace, workload, or node with no finding reads
  "No finding" instead of Healthy.
- On the report screen, the state of a node that is not ready or unknown
  carries its mark.
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
