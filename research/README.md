# TunnelForge research paper and reproducibility artifact

The six-page PDF is `output/pdf/tunnelforge_ieee_research_paper.pdf`.
It uses US Letter, two columns, Times typography, numbered sections and references,
three vector figures and four tables. The supplied institution name is used as
provided; no campus, city, email address or conference affiliation is invented.

Authors: Soham Agarwal (202452332), Prem Sharma (202452328), and Aghnivesh
Thottumkara (202451175), Department of Information Technology, Indian Institute
of Information Technology. Supervisor: Dr. Rishabh Gupta.

## Files

- `research/manuscript.md`: editable manuscript and all nine references.
- `research/build_paper.py`: PDF renderer and IEEEtran source exporter.
- `research/experiment/main.go`: reproducible Go workload generator.
- `output/research/data/results.json`: original run-level results and environment.
- `output/research/data/latencies.csv`: all 9,216 measured request observations.
- `output/research/tunnelforge_ieee.tex`: editable IEEEtran conference source.
- `output/research/figures/`: vector PDFs referenced by the LaTeX source.

## Repeat the experiment

From the repository root:

```bash
go run ./research/experiment -out output/research/repeated-data
```

This generates new measurements without replacing the published dataset. Results
vary with host and background activity. To publish a new run, update the selected
dataset in the builder and reconcile every numerical statement in the manuscript;
do not regenerate tables alone while leaving old narrative values unchanged.

The experiment makes loopback TCP connections and must be allowed to bind local
ports. It uses POST echo requests of 1 KiB and 64 KiB with concurrency 1, 8 and 32;
each mode/configuration has three measured repetitions of 256 requests after 32
excluded warm-ups. All services run in one process. The source alternates mode
order by repeat; it does not randomize or fully balance order. Each request is
validated by status and exact byte comparison. This is not a WAN/TLS benchmark.

## Rebuild the PDF

Install ReportLab in a Python environment and run:

```bash
python3 research/build_paper.py
```

The renderer reads the selected original dataset and the Markdown manuscript.
Explicit page divisions maintain the six-page layout. Rendering is separate from
LaTeX: the exported IEEEtran source is an editable alternative for a college or
conference template, and its pagination can differ with TeX packages and fonts.

Upload `tunnelforge_ieee.tex` and the adjacent `figures` directory into an
IEEEtran-enabled LaTeX project, select that file as the main document, and compile.
No journal title, conference name, copyright line, DOI, acceptance status or
publication claim is assigned to this student manuscript.
