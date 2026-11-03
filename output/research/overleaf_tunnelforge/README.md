# TunnelForge: self-contained IEEE Overleaf project

## Open in Overleaf

1. Choose **New Project > Upload Project**.
2. Upload `tunnelforge_overleaf.zip`.
3. Select `main.tex` as the main document.
4. Select **pdfLaTeX** as the compiler and a current TeX Live version.
5. Recompile. Allow LaTeX to finish its reruns for cross-references.

You can instead create a blank project and paste the entire `main.tex` into
Overleaf's `main.tex`. No other file is needed to compile the paper.

## Included in main.tex

- Official IEEEtran conference class in US Letter, two-column layout.
- Team members, roll numbers, department, supplied IIIT institution name,
  and supervision by Dr. Rishabh Gupta.
- Abstract, index terms, research questions, related work, system model,
  implementation, evaluation, measured results, threats to validity, conclusion.
- Native TikZ architecture diagram and message sequence diagram.
- Native PGFPlots throughput chart with distinct grayscale marker shapes and
  observed min-max ranges. Those ranges are not confidence intervals.
- Four native tables; no raster screenshots or imported table images.
- Numbered equations for Base64 size, ownership, throughput and quantiles.
- Nine bibliography entries and automatic citation/reference numbering.

No image uploads, separate .bib file, BibTeX step or shell escape is required.
IEEEtran, TikZ and PGFPlots are standard Overleaf packages.

## Editing and layout

Architecture and sequence diagrams span both columns to keep text readable.
IEEEtran may place these figures at the top of a later page. That is normal;
`\ref` keeps figure numbers correct. Avoid replacing `figure*` with a narrow
single-column figure or shrinking the entire diagram as an image.

The source preserves IEEEtran's body size, margins and caption formatting.
There is no forced margin override or hard-coded page count. This is a revised
source, not the exact renderer of the earlier six-page PDF. Its final page count
must be checked in the compiler; figure placement and TeX Live versions can
change pagination. If a college provides its own IEEE template, follow that
template's author/affiliation and page-limit requirements.

No campus, email, city, conference venue, acceptance status or DOI for the
student manuscript has been invented. The institution name is the one supplied
by the team; add the campus/city if required by the college.

## Measurement provenance

The numerical claims are from the saved 29 October 2026 experiment:

- macOS 27.0 arm64, Go 1.26.5, 10 logical CPUs, GOMAXPROCS=10.
- Same-process services communicating through real loopback TCP/WebSocket.
- Payloads 1 and 64 KiB; closed-loop concurrency 1, 8 and 32.
- Direct/tunnel modes; 3 repetitions; 256 measured and 32 warm-up requests/run.
- 36 measured runs; 9,216 measured requests; zero errors or byte mismatches.
- Reported p50, p95 and rates are medians of run-level metrics.
- No TLS, WAN, public-cloud, NAT-traversal or competitor performance experiment.

Original measurements and experiment source remain in the repository:
`output/research/data/`, `research/experiment/main.go`.
Do not describe the loopback rates as Internet or production throughput.
