# slurmy

A terminal UI for SLURM. Browse your completed and running jobs, read queued jobs, and tail stdout — all without leaving the terminal.

![demo2](https://github.com/user-attachments/assets/98c96feb-2544-4916-95cd-2cfe42157216)

## Features

- **Tabbed interface** — switch between **Jobs**, **Cluster**, and **Users** with `tab` or `1`/`2`/`3`
- **Cluster tab** — at-a-glance GPU summary (used / free / down) plus a colour-coded grid of every node showing its state, per-GPU allocation bar, and CPU usage (data from `sinfo`)
- **Users tab** — ranked bar chart of how many GPUs each user is running (and how many they have queued), aggregated from `squeue` across the whole cluster
- **Job list** — shows jobs from the last 30 days via `sacct`, refreshed every 2 seconds
- **Pending jobs at the top** — pulls queued jobs from `squeue` and prepends them with their wait reason (e.g. `Resources`, `Priority`)
- **Live stdout tail** — right-hand panel tails the job's stdout file like `tail -f`, updating every second
- **Large file support** — reads files backwards in 64 KB chunks (same strategy as `tail`), so multi-GB log files should open instantly
- **ANSI & progress bar stripping** — cleans up tqdm bars and escape codes so the TUI stays readable
- **SLURM path variables** — resolves `%u`, `%j`, `%J`, `%A`, `%a` in stdout paths automatically
- **Job details** — shows job ID, name, user, account, start time, elapsed time, allocated CPUs, TRES, node list, and stdout path
- **Cancel jobs** — press `c` to cancel the selected job (with confirmation prompt)

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

## License

MIT
