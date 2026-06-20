# slurmy

A terminal UI for SLURM. Browse your completed and running jobs, read queued jobs, and tail stdout — all without leaving the terminal.

![demo2](https://github.com/user-attachments/assets/98c96feb-2544-4916-95cd-2cfe42157216)

## Features

- **Three tabs** — **Jobs**, **Cluster**, and **Users**, switched with `tab` or `1`/`2`/`3`
- **Jobs** — your jobs from the last 30 days (`sacct`) with pending jobs and their wait reason pinned to the top (`squeue`); cancel the selected job with `c`
- **Live stdout/stderr tail** — follows the selected job's output like `tail -f`; press `o`/`e` to switch stream and `enter` to scroll back through the log. Handles multi-GB files and strips ANSI / tqdm progress bars
- **Cluster** — at-a-glance GPU summary (used / free / down) plus a colour-coded grid of every node with its state, per-GPU allocation, and CPU usage (`sinfo`)
- **Users** — ranked bar chart of GPUs each user is running and has queued, across the whole cluster (`squeue`)

## Keybindings

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle between tabs (Jobs / Cluster / Users) |
| `1` / `2` / `3` | Jump straight to a tab |
| `↑`/`↓` or `j`/`k` | Navigate job list / scroll the active tab |
| `c` | Cancel selected job (Jobs tab, running/pending only) |
| `y` | Confirm cancellation |
| `n` or `Esc` | Dismiss confirmation |
| `q` or `Ctrl+C` | Quit |

## Requirements

- A SLURM environment with `sacct`, `squeue`, and `sinfo` in `$PATH`
- Terminal with 256-colour support

No Go toolchain needed on the cluster — just copy the pre-built binary.

## Installation

### Download a release binary (recommended)

Grab the latest binary from the [Releases](../../releases) page and copy it to your cluster:

```bash
# Linux amd64 (most clusters)
wget -O slurmy https://github.com/bertaveira/slurmy/releases/latest/download/slurmy-linux-amd64
chmod +x slurmy
./slurmy
```

### Add to your PATH (optional)

Move the binary to a directory in your `$PATH`, or add an alias to your shell config:

```bash
# Option 1: Move to ~/bin (create it if needed)
mkdir -p ~/bin
mv slurmy ~/bin/

# Option 2: Add an alias to your .bashrc
echo 'alias slurmy="~/path/to/slurmy"' >> ~/.bashrc
source ~/.bashrc
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

## Demo mode

Run with `--demo` to launch the UI on synthetic data — no SLURM, no cluster, no
network. Useful for screenshots, recordings, or just trying it out:

```bash
slurmy --demo
```

It shows a made-up set of jobs (with a live-streaming log), a fake node grid,
and fake per-user GPU usage. Synthetic log files are written under
`/tmp/slurmy-demo/`.

## License

MIT
