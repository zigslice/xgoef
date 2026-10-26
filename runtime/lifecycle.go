package runtime

// Lifecycle is one host lifecycle event.
// Ready and exit have an empty label and URL.
// Page-load carries the window label and the main-frame URL from the browser.
// The page cannot send these events.
type Lifecycle struct {
	Event string
	Label string
	URL   string
}

const (
	// LifecycleReady runs after the first browser is created and before the message loop.
	LifecycleReady = "ready"
	// LifecycleExit runs after the message loop returns and before plugin teardown.
	LifecycleExit = "exit"
	// LifecyclePageLoad runs when a main frame finishes loading.
	LifecyclePageLoad = "page-load"
)

const (
	pageLoadLabelMax = 64
	pageLoadURLMax   = 4096
)

// OnLifecycle registers a host callback.
// Callbacks run in registration order. A nil callback is ignored.
// Page-load callbacks run on the CEF UI thread.
func (a *App) OnLifecycle(fn func(Lifecycle)) {
	if a == nil || fn == nil {
		return
	}
	a.lifeMu.Lock()
	a.life = append(a.life, fn)
	a.lifeMu.Unlock()
}

func (a *App) emitLifecycle(ev Lifecycle) {
	if a == nil {
		return
	}
	a.lifeMu.Lock()
	hooks := append([]func(Lifecycle){}, a.life...)
	a.lifeMu.Unlock()
	for _, fn := range hooks {
		if fn != nil {
			fn(ev)
		}
	}
}

func (a *App) watchPageLoad() {
	if a == nil || a.Bridge == nil {
		return
	}
	a.Bridge.SetPageLoad(func(label, url string) {
		if !acceptPageLoad(label, url) {
			return
		}
		a.emitLifecycle(Lifecycle{Event: LifecyclePageLoad, Label: label, URL: url})
	})
}

func acceptPageLoad(label, url string) bool {
	return pageLoadTextOK(label, pageLoadLabelMax) && pageLoadTextOK(url, pageLoadURLMax)
}

func pageLoadTextOK(text string, max int) bool {
	if text == "" || len(text) > max {
		return false
	}
	for _, r := range text {
		if r < 0x20 {
			return false
		}
	}
	return true
}
