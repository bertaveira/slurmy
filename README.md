# slurmy

A terminal UI for SLURM. Browse your completed and running jobs, read queued jobs, and tail stdout — all without leaving the terminal.

![demo placeholder](https://img.shields.io/badge/TUI-SLURM-blue)

## Features

- **Job list** — shows jobs from the last 30 days via `sacct`, refreshed every 2 seconds
- **Pending jobs at the top** — pulls queued jobs from `squeue` and prepends them with their wait reason (e.g. `Resources`, `Priority`)
- **Live stdout tail** — right-hand panel tails the job's stdout file like `tail -f`, updating every second
- **Large file support** — reads files backwards in 64 KB chunks (same strategy as `tail`), so multi-GB log files should open instantly
- **ANSI & progress bar stripping** — cleans up tqdm bars and escape codes so the TUI stays readable
- **SLURM path variables** — resolves `%u`, `%j`, `%J`, `%A`, `%a` in stdout paths automatically
- **Job details** — shows job ID, name, user, account, start time, elapsed time, allocated CPUs, TRES, node list, and stdout path

## Requirements

- A SLURM environment with `sacct` and `squeue` in `$PATH`
- Terminal with 256-colour support

No Go toolchain needed on the cluster — just copy the pre-built binary.

## Installation

### Download a release binary (recommended)

Grab the latest binary from the [Releases](../../releases) page and copy it to your cluster:

```bash
# Linux amd64 (most clusters)
wget https://github.com/bertaveira/slurmy/releases/latest/download/slurmy-linux-amd64
chmod +x slurmy-linux-amd64
./slurmy-linux-amd64
```

### Build from source

```bash
git clone https://github.com/bertaveira/slurmy
cd slurmy
go build -o slurmy .
./slurmy
```

**Cross-compile from macOS to Linux:**

```bash
make build-linux-amd64   # → slurmy-linux-amd64
make build-linux-arm64   # → slurmy-linux-arm64
make build-all           # both architectures
```

Then `scp` the binary to your cluster and run it.

## License

MIT
