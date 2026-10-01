package tailer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/config"
	"github.com/aribrilliantsyah/kapture/internal/model"
	badgerstore "github.com/aribrilliantsyah/kapture/internal/storage/badger"
)

const cid = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

type fixture struct {
	t      *testing.T
	target string // the real file, like /var/log/pods/.../0.log
	tailer *Tailer
	n      int
}

// newFixture mimics a node: /var/log/containers/*.log symlinks pointing at
// files under /var/log/pods.
func newFixture(t *testing.T) *fixture {
	root := t.TempDir()
	podDir := filepath.Join(root, "pods", "prod_api-7f8b9c6d4-x2k1p_uid", "app")
	containers := filepath.Join(root, "containers")
	for _, d := range []string{podDir, containers} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &fixture{t: t, target: filepath.Join(podDir, "0.log")}
	f.touch()
	link := filepath.Join(containers, "api-7f8b9c6d4-x2k1p_prod_app-"+cid+".log")
	if err := os.Symlink(f.target, link); err != nil {
		t.Fatal(err)
	}

	store, err := badgerstore.New(badgerstore.Options{Path: filepath.Join(root, "db"), Compression: "none", GCInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	f.tailer = New(config.AgentConfig{LogPath: containers, Collector: config.CollectorConfig{BatchSize: 10}}, "node-1", store, nil)
	t.Cleanup(func() { store.Close() })
	return f
}

func (f *fixture) touch() {
	file, err := os.Create(f.target)
	if err != nil {
		f.t.Fatal(err)
	}
	file.Close()
}

// write appends CRI lines to the real file (not through the symlink dir).
func (f *fixture) write(lines ...string) {
	file, err := os.OpenFile(f.target, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatal(err)
	}
	defer file.Close()
	for _, l := range lines {
		fmt.Fprintln(file, l)
	}
}

func (f *fixture) line(stream, tag, msg string) string {
	f.n++
	ts := time.Now().UTC().Add(time.Duration(f.n) * time.Millisecond).Format(time.RFC3339Nano)
	return ts + " " + stream + " " + tag + " " + msg
}

// expect reads entries until the wanted messages arrived, failing on anything else.
func (f *fixture) expect(want ...string) []model.LogEntry {
	f.t.Helper()
	var got []model.LogEntry
	deadline := time.After(5 * time.Second)
	for len(got) < len(want) {
		select {
		case e := <-f.tailer.Out():
			got = append(got, e)
		case <-deadline:
			f.t.Fatalf("timed out: got %d of %d entries (%v)", len(got), len(want), got)
		}
	}
	for i, e := range got {
		if e.Message != want[i] {
			f.t.Fatalf("entry %d = %q, want %q", i, e.Message, want[i])
		}
	}
	select {
	case e := <-f.tailer.Out():
		f.t.Fatalf("unexpected extra entry %q", e.Message)
	case <-time.After(1500 * time.Millisecond):
	}
	return got
}

func TestTailerFollowsSymlinkedFile(t *testing.T) {
	f := newFixture(t)
	f.write(f.line("stdout", "F", "INFO first"), f.line("stderr", "F", "ERROR second"))
	if err := f.tailer.Start(); err != nil {
		t.Fatal(err)
	}
	defer f.tailer.Stop()

	got := f.expect("INFO first", "ERROR second")
	e := got[1]
	if e.Namespace != "prod" || e.Pod != "api-7f8b9c6d4-x2k1p" || e.Workload != "api" || e.Container != "app" ||
		e.Level != "ERROR" || e.Stream != "stderr" || e.Node != "node-1" || e.Seq == 0 {
		t.Fatalf("bad metadata: %+v", e)
	}

	// Writes to the symlink target produce no inotify event in the
	// containers dir; polling must still pick them up.
	f.write(f.line("stdout", "F", "third"))
	f.expect("third")

	// CRI partial lines are joined; indented lines join the previous entry.
	f.write(
		f.line("stdout", "P", "long "),
		f.line("stdout", "F", "line"),
		f.line("stdout", "F", "java.lang.IllegalStateException: boom"),
		f.line("stdout", "F", "\tat com.example.Main.run(Main.java:10)"),
	)
	f.expect("long line", "java.lang.IllegalStateException: boom\n\tat com.example.Main.run(Main.java:10)")

	// A half-written line is not emitted until its newline arrives.
	half := f.line("stdout", "F", "split write")
	file, _ := os.OpenFile(f.target, os.O_APPEND|os.O_WRONLY, 0o644)
	file.WriteString(half[:20])
	file.Close()
	time.Sleep(1500 * time.Millisecond)
	file, _ = os.OpenFile(f.target, os.O_APPEND|os.O_WRONLY, 0o644)
	file.WriteString(half[20:] + "\n")
	file.Close()
	f.expect("split write")

	// kubelet rotation: last lines of the old file, then the new file.
	f.write(f.line("stdout", "F", "before rotate"))
	if err := os.Rename(f.target, f.target+".20250115"); err != nil {
		t.Fatal(err)
	}
	f.touch()
	f.write(f.line("stdout", "F", "after rotate"))
	f.expect("before rotate", "after rotate")
}

func TestTailerResumesFromOffset(t *testing.T) {
	f := newFixture(t)
	f.write(f.line("stdout", "F", "one"))
	if err := f.tailer.Start(); err != nil {
		t.Fatal(err)
	}
	f.expect("one")
	f.tailer.Stop()

	f.write(f.line("stdout", "F", "two"))
	restarted := New(f.tailer.cfg, "node-1", f.tailer.store, nil)
	f.tailer = restarted
	if err := restarted.Start(); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop()
	if got := f.expect("two"); !strings.HasPrefix(got[0].Date, "20") {
		t.Fatalf("bad date %q", got[0].Date)
	}
}
