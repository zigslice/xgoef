package notification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestPermissionAndShow(t *testing.T) {
	base := t.TempDir()
	icon := filepath.Join(base, "icon.png")
	if err := os.WriteFile(icon, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(base), "outside.png")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var notes []Note
	p, err := New(base, filepath.Join(base, "web"), "xgoef-hello", []string{"$APP"}, nil, func(n Note) error {
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Invoke("notification:is_permission_granted", []byte(`{}`))
	if err != nil || string(got) != "false" {
		t.Fatalf("before %s %v", got, err)
	}
	if _, err := r.Invoke("notification:show", []byte(`{"title":"hi"}`)); err == nil || ipc.CodeOf(err) != "notification.denied" || len(notes) != 0 {
		t.Fatalf("early show %v notes %d", err, len(notes))
	}
	got, err = r.Invoke("notification:request_permission", []byte(`{}`))
	if err != nil || string(got) != `"granted"` {
		t.Fatalf("request %s %v", got, err)
	}
	got, err = r.Invoke("notification:is_permission_granted", []byte(`{}`))
	if err != nil || string(got) != "true" {
		t.Fatalf("after %s %v", got, err)
	}
	if _, err := r.Invoke("notification:show", []byte(`{"title":" hello ","body":"line\n2","icon":"icon.png"}`)); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].AppName != "xgoef-hello" || notes[0].Title != "hello" || notes[0].Body != "line\n2" || notes[0].Icon != icon {
		t.Fatalf("%+v", notes)
	}
	if _, err := r.Invoke("notification:show", []byte(`{"title":"x","icon":"`+outside+`"}`)); err == nil || ipc.CodeOf(err) != "notification.icon" || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("icon %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("rejected icon was sent: %+v", notes)
	}
}

func TestShowValidation(t *testing.T) {
	p, err := New(t.TempDir(), "", "", nil, nil, func(Note) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("notification:request_permission", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{
		`{}`,
		`{"title":"  "}`,
		`{"title":"a\nb"}`,
		`{"title":1}`,
		`{"title":"ok","urgency":"critical"}`,
		`{"title":"` + strings.Repeat("a", maxTitle+1) + `"}`,
		`[]`,
	} {
		if _, err := r.Invoke("notification:show", []byte(args)); err == nil || ipc.CodeOf(err) != "notification.invalid" {
			t.Fatalf("%s: %v", args, err)
		}
	}
	if _, err := r.Invoke("notification:request_permission", []byte(`{"yes":true}`)); err == nil || ipc.CodeOf(err) != "notification.invalid" {
		t.Fatal(err)
	}
}

func TestDeniedWhenServiceMissing(t *testing.T) {
	p, err := New(t.TempDir(), "", "bad\nname", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.SetProbe(func() bool { return false })
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Invoke("notification:request_permission", []byte(`{}`))
	if err != nil || string(got) != `"denied"` {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := r.Invoke("notification:show", []byte(`{"title":"hi"}`)); err == nil || ipc.CodeOf(err) != "notification.denied" {
		t.Fatal(err)
	}
}

func TestSenderFailure(t *testing.T) {
	p, err := New(t.TempDir(), "", "", nil, nil, func(Note) error {
		return os.ErrPermission
	})
	if err != nil {
		t.Fatal(err)
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("notification:request_permission", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("notification:show", []byte(`{"title":"hi"}`))
	if err == nil || ipc.CodeOf(err) != "notification.show" || !strings.Contains(err.Error(), "permission") {
		t.Fatal(err)
	}
}

func TestNotifyArgs(t *testing.T) {
	got := notifyArgs(Note{AppName: "xgoef", Title: "-n", Body: "hi", Icon: "/tmp/icon.png"})
	if strings.Join(got, "\x00") != strings.Join([]string{"-a", "xgoef", "-i", "/tmp/icon.png", "--", "-n", "hi"}, "\x00") {
		t.Fatalf("%q", got)
	}
	got = notifyArgs(Note{Title: "only"})
	if strings.Join(got, " ") != "-a xgoef -- only" {
		t.Fatalf("%q", got)
	}
}

var _ plugin.Host = host{}
