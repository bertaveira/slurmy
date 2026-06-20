package slurm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// demoDir is where synthetic log files are written so the stdout/stderr tail
// has real files to read in demo mode.
const demoDir = "/tmp/slurmy-demo"

// NewDemoClient builds a Client backed entirely by synthetic data — no SLURM
// binaries are invoked. It writes fake log files to disk so the output tail
// works, and starts a background goroutine that appends to the "live" job so a
// recording shows output streaming in. Use this for screenshots and demos.
func NewDemoClient() *Client {
	c := &Client{
		Username:        "researcher",
		AvailableFields: make(map[string]bool),
		Demo:            true,
	}
	for _, f := range desiredFields {
		c.AvailableFields[f] = true
	}

	_ = os.MkdirAll(demoDir, 0o755)

	c.demoJobs = demoJobs()
	c.demoNodes = demoNodes()
	c.demoSum = summarize(c.demoNodes)
	c.demoUsage = demoUsage()

	c.startDemoAppender(filepath.Join(demoDir, "train_diffusion_v3.out"))
	return c
}

// logPath writes content to a file named after the job in the demo dir and
// returns its path. An empty content string means "no separate file".
func logPath(name, content string) string {
	if content == "" {
		return ""
	}
	p := filepath.Join(demoDir, name)
	_ = os.WriteFile(p, []byte(content), 0o644)
	return p
}

// trainingLog renders a realistic-looking training log with epochs, a progress
// bar that uses carriage returns (to show the TUI's progress-bar stripping),
// and structured INFO lines.
func trainingLog(run string, epochs int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[09:23:01] INFO  Starting run: %s\n", run)
	fmt.Fprintf(&b, "[09:23:02] INFO  Loaded 1,281,167 samples · world_size=8 · global_batch=2048\n")
	fmt.Fprintf(&b, "[09:23:04] INFO  Model: 6.7B params · bf16 · ZeRO-2 · grad_ckpt=on\n")
	// A line with embedded carriage returns — the TUI keeps only the final state.
	b.WriteString("Resolving data shards:  0%|          \r 50%|█████     \r100%|██████████| 16/16 [00:08<00:00]\n")
	loss := 2.40
	for e := 1; e <= epochs; e++ {
		fmt.Fprintf(&b, "Epoch %d/50: 100%%|██████████| 2502/2502 [04:%02d<00:00, 9.91it/s, loss=%.4f]\n", e, 5+e%50, loss)
		fmt.Fprintf(&b, "[%02d:%02d:17] INFO  epoch=%d train_loss=%.4f val_loss=%.4f lr=1.0e-04 grad_norm=%.2f\n",
			9+(e/12), (24+e*2)%60, e, loss, loss*0.94, 1.8-float64(e)*0.03)
		loss *= 0.86
	}
	fmt.Fprintf(&b, "[12:41:55] INFO  checkpoint saved → checkpoints/%s/epoch_%d.pt\n", run, epochs)
	return b.String()
}

func demoJobs() []JobInfo {
	out := func(name string) string { return logPath(name+".out", trainingLog(name, 6)) }

	failErr := strings.Join([]string{
		"Traceback (most recent call last):",
		`  File "train.py", line 214, in <module>`,
		"    main(cfg)",
		`  File "train.py", line 167, in main`,
		"    loss.backward()",
		`  File "torch/_tensor.py", line 522, in backward`,
		"    torch.autograd.backward(self, gradient, retain_graph, create_graph)",
		"torch.cuda.OutOfMemoryError: CUDA out of memory. Tried to allocate 2.00 GiB",
		"  (GPU 0; 79.15 GiB total; 77.40 GiB reserved; 1.21 GiB free)",
	}, "\n") + "\n"

	runWarn := strings.Join([]string{
		"[09:23:00] WARNING  NCCL_DEBUG=INFO not set; topology autodetect may be slow",
		"[09:23:03] WARNING  flash-attn not found, falling back to memory-efficient SDPA",
	}, "\n") + "\n"

	jobs := []JobInfo{
		{
			JobID: "184213", JobName: "train_llm_70b", User: "researcher", Account: "ml-research",
			State: Pending, StartTime: "N/A", ElapsedTime: "0:00", TimeLimit: "3-00:00:00",
			AllocCPUS: "112", AllocTRES: "cpu=112,gres/gpu=8", Reason: "Resources",
		},
		{
			JobID: "184219", JobName: "sweep_lr_3e4", User: "researcher", Account: "ml-research",
			State: Pending, StartTime: "N/A", ElapsedTime: "0:00", TimeLimit: "12:00:00",
			AllocCPUS: "32", AllocTRES: "cpu=32,gres/gpu=2", Reason: "Priority",
		},
		{
			JobID: "184007", JobName: "train_diffusion_v3", User: "researcher", Account: "ml-research",
			State: Running, StartTime: "2026-06-20T09:23:01", ElapsedTime: "3:18:42", TimeLimit: "1-00:00:00",
			AllocCPUS: "96", AllocTRES: "cpu=96,gres/gpu=8", NodeList: "gpu-a02",
			StdOut: out("train_diffusion_v3"),
			StdErr: logPath("train_diffusion_v3.err", runWarn),
		},
		{
			JobID: "184103", JobName: "eval_benchmark", User: "researcher", Account: "ml-research",
			State: Running, StartTime: "2026-06-20T11:05:12", ElapsedTime: "1:36:31", TimeLimit: "06:00:00",
			AllocCPUS: "16", AllocTRES: "cpu=16,gres/gpu=1", NodeList: "gpu-a07",
			StdOut: out("eval_benchmark"),
		},
		{
			JobID: "184150", JobName: "finetune_sft", User: "researcher", Account: "ml-research",
			State: Running, StartTime: "2026-06-20T12:01:44", ElapsedTime: "0:39:59", TimeLimit: "1-00:00:00",
			AllocCPUS: "48", AllocTRES: "cpu=48,gres/gpu=4", NodeList: "gpu-a10",
			StdOut: out("finetune_sft"),
		},
		{
			JobID: "183990", JobName: "preprocess_dataset", User: "researcher", Account: "ml-research",
			State: Completed, StartTime: "2026-06-20T07:14:08", ElapsedTime: "0:52:10", TimeLimit: "04:00:00",
			AllocCPUS: "64", AllocTRES: "cpu=64", NodeList: "cpu-02",
			StdOut: out("preprocess_dataset"),
		},
		{
			JobID: "183902", JobName: "train_gan_v2", User: "researcher", Account: "ml-research",
			State: Failed, StartTime: "2026-06-20T02:30:00", ElapsedTime: "0:04:51", TimeLimit: "1-00:00:00",
			AllocCPUS: "24", AllocTRES: "cpu=24,gres/gpu=2", NodeList: "gpu-a05",
			StdOut: out("train_gan_v2"),
			StdErr: logPath("train_gan_v2.err", failErr),
		},
		{
			JobID: "183871", JobName: "hparam_search", User: "researcher", Account: "ml-research",
			State: Canceled, StartTime: "2026-06-19T22:11:30", ElapsedTime: "2:09:00", TimeLimit: "1-00:00:00",
			AllocCPUS: "32", AllocTRES: "cpu=32,gres/gpu=4", NodeList: "gpu-a13",
			StdOut: out("hparam_search"),
		},
		{
			JobID: "183840", JobName: "tokenize_corpus", User: "researcher", Account: "ml-research",
			State: Completed, StartTime: "2026-06-19T18:42:00", ElapsedTime: "1:18:22", TimeLimit: "06:00:00",
			AllocCPUS: "96", AllocTRES: "cpu=96", NodeList: "cpu-04",
			StdOut: out("tokenize_corpus"),
		},
	}
	return jobs
}

func demoNodes() []NodeInfo {
	const gpuType = "A100-80GB"
	gpu := func(name, state string, used, cpuAlloc int) NodeInfo {
		return NodeInfo{
			Name: name, State: state, Partition: "gpu",
			GPUTotal: 8, GPUUsed: used, GPUType: gpuType,
			CPUAlloc: cpuAlloc, CPUTotal: 128,
			MemTotalMB: 1031000, MemFreeMB: 512000,
		}
	}
	cpu := func(name, state string, cpuAlloc int) NodeInfo {
		return NodeInfo{
			Name: name, State: state, Partition: "cpu",
			CPUAlloc: cpuAlloc, CPUTotal: 128,
			MemTotalMB: 515000, MemFreeMB: 256000,
		}
	}

	return []NodeInfo{
		gpu("gpu-a01", "mixed", 6, 92),
		gpu("gpu-a02", "allocated", 8, 128),
		gpu("gpu-a03", "mixed", 4, 64),
		gpu("gpu-a04", "idle", 0, 0),
		gpu("gpu-a05", "allocated", 8, 128),
		gpu("gpu-a06", "mixed", 7, 104),
		gpu("gpu-a07", "mixed", 2, 28),
		gpu("gpu-a08", "idle", 0, 0),
		gpu("gpu-a09", "allocated", 8, 128),
		gpu("gpu-a10", "mixed", 5, 72),
		gpu("gpu-a11", "drained", 0, 0),
		gpu("gpu-a12", "mixed", 3, 40),
		gpu("gpu-a13", "allocated", 8, 128),
		gpu("gpu-a14", "idle", 0, 0),
		gpu("gpu-a15", "down", 0, 0),
		gpu("gpu-a16", "mixed", 6, 88),
		cpu("cpu-01", "mixed", 64),
		cpu("cpu-02", "allocated", 128),
		cpu("cpu-03", "idle", 0),
		cpu("cpu-04", "mixed", 96),
		cpu("cpu-05", "idle", 0),
		cpu("cpu-06", "drained", 0),
	}
}

func demoUsage() []UserUsage {
	// Pre-sorted by running GPUs descending (as GetUserUsage would return).
	return []UserUsage{
		{User: "alice", RunningGPUs: 16, RunningJobs: 4, RunningCPUs: 192, PendingGPUs: 8, PendingJobs: 1},
		{User: "bob", RunningGPUs: 12, RunningJobs: 3, RunningCPUs: 144, PendingGPUs: 0, PendingJobs: 0},
		{User: "carol", RunningGPUs: 8, RunningJobs: 1, RunningCPUs: 96, PendingGPUs: 16, PendingJobs: 2},
		{User: "researcher", RunningGPUs: 8, RunningJobs: 3, RunningCPUs: 160, PendingGPUs: 10, PendingJobs: 2},
		{User: "dave", RunningGPUs: 6, RunningJobs: 6, RunningCPUs: 84, PendingGPUs: 0, PendingJobs: 0},
		{User: "erin", RunningGPUs: 4, RunningJobs: 2, RunningCPUs: 56, PendingGPUs: 2, PendingJobs: 1},
		{User: "frank", RunningGPUs: 2, RunningJobs: 2, RunningCPUs: 28, PendingGPUs: 0, PendingJobs: 0},
		{User: "grace", RunningGPUs: 1, RunningJobs: 1, RunningCPUs: 14, PendingGPUs: 4, PendingJobs: 2},
	}
}

// startDemoAppender appends a new epoch line to the live job's stdout file every
// ~900ms so a recording shows output streaming in like `tail -f`.
func (c *Client) startDemoAppender(path string) {
	go func() {
		epoch := 7
		loss := 0.42
		for {
			time.Sleep(900 * time.Millisecond)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
			if err != nil {
				return
			}
			fmt.Fprintf(f, "Epoch %d/50: 100%%|██████████| 2502/2502 [04:%02d<00:00, 9.91it/s, loss=%.4f]\n", epoch, epoch%60, loss)
			fmt.Fprintf(f, "[%02d:%02d:17] INFO  epoch=%d train_loss=%.4f val_loss=%.4f lr=1.0e-04\n",
				12+epoch/12, epoch%60, epoch, loss, loss*0.95)
			_ = f.Close()
			epoch++
			loss *= 0.97
		}
	}()
}
