#import "../../../graphite/lib.typ": graphite-report

// Proves that a report entirely in English is possible. Before F2 the
// labels translated and the rest did not: the text hyphenated as Portuguese
// and the date came out in pt-BR order.
#show: graphite-report.with(
  title: [Monthly report],
  subtitle: [Consolidated results],
  author: "Operations team",
  date: datetime(year: 2026, month: 7, day: 31),
  labels: (author: [Owner], date: [Issued]),
  theme: (
    lang: "en",
    region: "US",
    date-format: "[month padding:zero]/[day padding:zero]/[year]",
  ),
)

= Overview

The period closed with operational stability.
