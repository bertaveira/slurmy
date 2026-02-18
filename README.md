# slurmy

A small TUI for SLURM: browse your jobs and tail their stdout in one place.

**Requirements:** Go 1.24+, a Linux environment with SLURM and `sacct` in your PATH.

## Build & run

```bash
go build -o slurmy .
./slurmy
```

For a cluster (cross-compile from macOS):

```bash
make build-linux-amd64   # or build-linux-arm64 / build-all
# then copy slurmy-linux-amd64 to the cluster and run it
```

## Usage

- **Left:** List of your jobs (last 30 days), refreshed every second.
- **Right:** Job details and a live tail of the job’s stdout file (updates every second).
- **Keys:** `↑/↓` to change selection, `q` or `Ctrl+C` to quit.

Stdout paths with SLURM patterns (`%u`, `%j`, `%A`, `%a`, `%J`) are resolved automatically. ANSI and progress bars (e.g. tqdm) are stripped so the TUI stays readable.

## License

MIT (or your choice).
